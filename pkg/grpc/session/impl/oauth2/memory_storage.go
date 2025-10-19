package oauth2

import (
	"time"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session"
)

type (
	inMemoryStorage struct {
		storeCfg   *oauth2SessionStoreConfig
		sessionCfg *session.Config

		sessions  map[string]*sessionImpl
		log       *log.Logger
		refresher map[string]*myRefresher
	}
)

var _ sessionStorage = (*inMemoryStorage)(nil)

//nolint:whitespace // editor/linter issue
func newInMemoryStorage(
	sessionCfg *session.Config,
	storeCfg *oauth2SessionStoreConfig,
) sessionStorage {
	return &inMemoryStorage{
		storeCfg:   storeCfg,
		sessionCfg: sessionCfg,
		sessions:   make(map[string]*sessionImpl),
		refresher:  make(map[string]*myRefresher),
		log:        log.Default().Named("grpc.session.oauth2.inmemory"),
	}
}

func (s *inMemoryStorage) Get(id string) (*sessionImpl, error) {
	if sess, ok := s.sessions[id]; ok {
		return sess, nil
	}
	return nil, session.ErrSessionNotFound
}

func (s *inMemoryStorage) Save(mySession *sessionImpl) error {
	mySession.raw.LastAccessed = time.Now()

	s.refresher[mySession.ID()] = &myRefresher{
		id:    mySession.ID(),
		timer: s.createRefresher(mySession.ID(), s.calcTokenRefreshWait(mySession.raw)),
	}
	s.sessions[mySession.ID()] = mySession
	return nil
}

func (s *inMemoryStorage) Delete(id string) error {
	delete(s.sessions, id)
	if _, ok := s.refresher[id]; ok {
		s.refresher[id].timer.Stop()
		delete(s.refresher, id)
	}
	return nil
}

func (s *inMemoryStorage) calcTokenRefreshWait(raw *sessionRawData) time.Duration {
	waitToRefresh := time.Until(raw.Token.Expiry) - s.storeCfg.refreshThreshold
	s.log.Debug("waiting to refresh token",
		log.String("id", raw.ID),
		log.Duration("waitToRefresh", waitToRefresh),
		log.Time("tokenExpiry", raw.Token.Expiry),
	)
	return waitToRefresh
}

//nolint:whitespace,funlen // ok here
func (s *inMemoryStorage) createRefresher(
	id string,
	waitToRefresh time.Duration,
) *time.Timer {
	return time.AfterFunc(waitToRefresh, func() {
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
			//nolint:errcheck // this instance won't produce errors here
			s.Delete(mySession.raw.ID)
			return
		}

		s.log.Debug("Refreshing token for session",
			log.String("id", mySession.raw.ID),
			log.Time("tokenExpiry", mySession.raw.Token.Expiry),
			log.Time("lastAccessed", mySession.raw.LastAccessed),
		)

		newToken, err := mySession.tokenSource.Token()
		if err != nil {
			s.log.Warn("error refreshing token from token source",
				log.String("id", mySession.raw.ID),
				log.ErrorField(err))
			//nolint:errcheck // this instance won't produce errors here
			s.Delete(mySession.raw.ID)
			return
		}
		s.log.Debug("updated token",
			log.Time("oldExpire", mySession.raw.Token.Expiry),
			log.Time("newExpire", newToken.Expiry))

		mySession.raw.Token = newToken

		s.refresher[id] = &myRefresher{
			id:    id,
			timer: s.createRefresher(id, s.calcTokenRefreshWait(mySession.raw)),
		}
	})
}
