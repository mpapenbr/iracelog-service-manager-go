package oauth2

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"golang.org/x/oauth2"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session"
)

type (
	// natsStorage is a session storage implementation backed by NATS JetStream.
	// nats is considered to be the source of truth.
	// In order not to overload nats there is also a local cache.
	// This is used to prevent every lastAccessed update to hit nats.
	//
	natsStorage struct {
		sessionCfg   *session.Config
		storeCfg     *oauth2SessionStoreConfig
		nc           *nats.Conn
		log          *log.Logger
		kv           jetstream.KeyValue
		kvLocks      jetstream.KeyValue
		cache        map[string]*mySession
		oauth2Config *oauth2.Config
		refresher    map[string]*myRefresher
	}
	mySession struct {
		si *sessionImpl
		t  time.Time // time when this entry entered the cache
	}
)

var _ sessionStorage = (*natsStorage)(nil)

//nolint:whitespace // editor/linter issue
func newNATSStorage(
	nc *nats.Conn,
	sessionCfg *session.Config,
	storeCfg *oauth2SessionStoreConfig,
) (sessionStorage, error) {
	ret := &natsStorage{
		sessionCfg: sessionCfg,
		storeCfg:   storeCfg,
		nc:         nc,
		log:        log.Default().Named("grpc.session.oauth2.nats"),
		cache:      make(map[string]*mySession),
		refresher:  make(map[string]*myRefresher),
	}
	ret.log.Debug("Initializing NATS storage for OAuth2 sessions")
	if err := ret.init(); err != nil {
		return nil, err
	}
	if err := ret.setupOAuth2Config(); err != nil {
		return nil, err
	}
	ret.setupCleanupLoop()
	ret.log.Debug("Initialized NATS storage for OAuth2 sessions")
	return ret, nil
}

func (s *natsStorage) init() error {
	var js jetstream.JetStream
	var err error
	if js, err = jetstream.New(s.nc); err != nil {
		return err
	}
	s.kv, err = js.CreateOrUpdateKeyValue(context.Background(), jetstream.KeyValueConfig{
		Bucket: "bff_sessions",
		TTL:    time.Minute * 2,
	})
	if err != nil {
		return err
	}
	s.kvLocks, err = js.CreateOrUpdateKeyValue(context.Background(),
		jetstream.KeyValueConfig{
			Bucket: "bff_sessions_locks",
			TTL:    time.Second * 5,
		})
	return err
}

func (s *natsStorage) setupOAuth2Config() error {
	if oidcProvider, err := oidc.NewProvider(
		context.Background(),
		s.storeCfg.oidcParam.IssuerURL); err != nil {
		return err
	} else {
		s.oauth2Config = &oauth2.Config{
			ClientID:     s.storeCfg.oidcParam.ClientID,
			ClientSecret: s.storeCfg.oidcParam.ClientSecret,
			Endpoint:     oidcProvider.Endpoint(),
		}
		return nil
	}
}

func (s *natsStorage) Get(id string) (*sessionImpl, error) {
	if s.cache[id] != nil {
		s.log.Debug("local cache hit", log.String("id", id))
		return s.cache[id].si, nil
	}

	sess, err := s.internalRead(id)
	if err != nil {
		return nil, err
	}
	// create sessionImpl from raw data

	ret := sessionImpl{
		raw:         sess,
		tokenSource: s.oauth2Config.TokenSource(context.Background(), sess.Token),
	}

	s.cache[id] = &mySession{
		si: &ret,
		t:  time.Now(),
	}
	return &ret, nil
}

func (s *natsStorage) Save(sess *sessionImpl) error {
	err := s.internalStore(sess.raw)
	if err == nil {
		s.cache[sess.raw.ID] = &mySession{
			si: sess,
			t:  time.Now(),
		}
	}
	s.refresher[sess.raw.ID] = &myRefresher{
		id:    sess.raw.ID,
		timer: s.createRefresher(sess.raw.ID, s.calcTokenRefreshWait(sess.raw)),
	}
	return err
}

func (s *natsStorage) Delete(id string) error {
	subj := s.composeKey(id)
	delete(s.cache, id)
	if _, ok := s.refresher[id]; ok {
		s.refresher[id].timer.Stop()
		delete(s.refresher, id)
	}
	return s.kv.Delete(context.Background(), subj)
}

// perform the actual store operation to nats.
// sessionRaw is serialized. uses internal key via composeKey() for storage
func (s *natsStorage) internalStore(sessionRaw *sessionRawData) error {
	subj := s.composeKey(sessionRaw.ID)
	sessionRaw.LastSync = time.Now()
	if idt, ok := sessionRaw.Token.Extra("id_token").(string); ok {
		sessionRaw.ExtraIDToken = idt
	}
	data, err := json.Marshal(sessionRaw)
	if err != nil {
		return err
	}
	_, err = s.kv.Put(context.Background(), subj, data)
	return err
}

// perform the actual store operation to nats.
// sessionRaw is serialized. uses internal key via composeKey() for storage
func (s *natsStorage) internalRead(id string) (*sessionRawData, error) {
	subj := s.composeKey(id)
	kve, err := s.kv.Get(context.Background(), subj)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, session.ErrSessionNotFound
		}
		return nil, err
	}

	var raw sessionRawData
	if err := json.Unmarshal(kve.Value(), &raw); err != nil {
		return nil, err
	}
	if raw.ExtraIDToken != "" {
		raw.Token = raw.Token.WithExtra(map[string]string{"id_token": raw.ExtraIDToken})
	}
	return &raw, nil
}

func (s *natsStorage) composeKey(id string) string {
	return "session.oauth2." + id
}

func (s *natsStorage) calcTokenRefreshWait(raw *sessionRawData) time.Duration {
	waitToRefresh := time.Until(raw.Token.Expiry) - s.storeCfg.refreshThreshold
	s.log.Debug("waiting to refresh token",
		log.String("id", raw.ID),
		log.Duration("waitToRefresh", waitToRefresh),
		log.Time("tokenExpiry", raw.Token.Expiry),
	)
	return waitToRefresh
}

func (s *natsStorage) setupCleanupLoop() {
	ticker := time.NewTicker(20 * time.Second)
	go func() {
		for range ticker.C {
			for k, v := range s.cache {
				raw, err := s.internalRead(k)

				//nolint:nestif // by design
				if err != nil {
					if errors.Is(err, session.ErrSessionNotFound) {
						continue
					}
					s.log.Warn("error reading session from nats", log.ErrorField(err))
					continue
				} else {
					log.Debug("cleanup data info",
						log.String("id", k),
						log.Time("tokenExp", raw.Token.Expiry),
						log.Time("lastAccessed", raw.LastAccessed),
						log.Time("lastSync", raw.LastSync),
					)
					if v.si.raw.LastAccessed.After(raw.LastAccessed) {
						// update local cache
						log.Debug("updating local cache for session",
							log.String("id", k))
						// we just update the lastAccessed from our cache entry
						// into the fresh data. Our cached data may contain no longer
						// usable token data
						raw.LastAccessed = v.si.raw.LastAccessed
						if sErr := s.internalStore(raw); sErr != nil {
							s.log.Warn("error updating session in nats",
								log.ErrorField(sErr))
						}
					}
				}
			}
			// forget old cache by creating a new cache
			s.cache = make(map[string]*mySession)
		}
	}()
}

func (s *natsStorage) lockEntry(id string) error {
	subj := "lock." + id
	_, err := s.kvLocks.Create(context.Background(), subj, []byte("locked"))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyExists) {
			return errors.New("could not acquire lock for session " + id)
		}
		return err
	}
	return nil
}

func (s *natsStorage) unlockEntry(id string) error {
	subj := "lock." + id
	return s.kvLocks.Delete(context.Background(), subj)
}

//nolint:whitespace,funlen // ok here
func (s *natsStorage) createRefresher(
	id string,
	waitToRefresh time.Duration,
) *time.Timer {
	return time.AfterFunc(waitToRefresh, func() {
		if err := s.lockEntry(id); err != nil {
			s.log.Debug("error locking session for refresher", log.ErrorField(err))
			return
		}
		defer func() {
			if err := s.unlockEntry(id); err != nil {
				s.log.Debug("error unlocking session for refresher", log.ErrorField(err))
			}
		}()
		mySession, err := s.Get(id)
		if err != nil {
			s.log.Warn("error getting session for refresher", log.ErrorField(err))
			return
		}
		if time.Since(mySession.raw.LastAccessed) > s.sessionCfg.Timeout {
			s.log.Debug("session expired, not refreshing token",
				log.String("id", mySession.raw.ID),
				log.Duration("sinceLast", time.Since(mySession.raw.LastAccessed)),
				log.Duration("timeout", s.sessionCfg.Timeout),
			)
			//nolint:errcheck // ignore error on delete
			s.Delete(mySession.raw.ID)
			return
		}

		s.log.Debug("Refreshing token for session",
			log.String("id", mySession.raw.ID),
			log.Time("tokenExpiry", mySession.raw.Token.Expiry),
			log.Time("lastAccessed", mySession.raw.LastAccessed),
			log.Time("lastSync", mySession.raw.LastSync),
		)

		myTS := s.oauth2Config.TokenSource(context.Background(), mySession.raw.Token)
		newToken, err := myTS.Token()
		if err != nil {
			s.log.Warn("error refreshing token from token source",
				log.String("id", mySession.raw.ID),
				log.ErrorField(err))
			if dErr := s.Delete(mySession.raw.ID); dErr != nil {
				s.log.Warn("error deleting session after token refresh failure",
					log.String("id", mySession.raw.ID),
					log.ErrorField(dErr))
			}
			return
		}
		s.log.Debug("updated token",
			log.Time("oldExpire", mySession.raw.Token.Expiry),
			log.Time("newExpire", newToken.Expiry))

		mySession.raw.Token = newToken
		if sErr := s.internalStore(mySession.raw); sErr != nil {
			s.log.Warn("error updating session in nats",
				log.String("id", mySession.raw.ID),
				log.ErrorField(sErr))
		}
		// TODO: release lock

		s.refresher[id] = &myRefresher{
			id:    id,
			timer: s.createRefresher(id, s.calcTokenRefreshWait(mySession.raw)),
		}
	})
}
