package nats

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/oidc"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/oidc/factory"
)

func New(common []oidc.Option, specific []Option) (oidc.PendingAuthStateCache, error) {
	cfg := &oidc.Config{
		Timeout: 30 * time.Second,
	}
	for _, o := range common {
		o(cfg)
	}
	ownCfg := &natsPASCacheConfig{}
	for _, o := range specific {
		o(ownCfg)
	}
	ret := &natsPASCache{
		cfg:    cfg,
		ownCfg: ownCfg,
		log:    log.Default().Named("grpc.oidc.nats_pending_auth_state_cache"),
	}
	ret.log.Debug("Initializing NATS storage for OAuth2 pending auth state cache")
	if err := ret.init(); err != nil {
		return nil, err
	}
	ret.log.Debug("Initialized NATS storage for OAuth2 pending auth state cache")
	return ret, nil
}

type (
	Option             func(*natsPASCacheConfig)
	natsPASCacheConfig struct {
		nc *nats.Conn
	}

	natsPASCache struct {
		cfg    *oidc.Config
		ownCfg *natsPASCacheConfig
		log    *log.Logger
		kv     jetstream.KeyValue
	}
)

var PendingAuthStateCacheTypeNats factory.CacheType = "nats"

func WithNATS(nc *nats.Conn) Option {
	return func(c *natsPASCacheConfig) {
		c.nc = nc
	}
}

func (s *natsPASCache) init() error {
	var js jetstream.JetStream
	var err error
	if js, err = jetstream.New(s.ownCfg.nc); err != nil {
		return err
	}
	s.kv, err = js.CreateOrUpdateKeyValue(context.Background(), jetstream.KeyValueConfig{
		Bucket: "oauth2_pending_logins",
		TTL:    s.cfg.Timeout,
	})
	if err != nil {
		return err
	}

	return err
}

func (s *natsPASCache) Get(id string) (*oidc.PendingAuthState, error) {
	kve, err := s.kv.Get(context.Background(), id)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, oidc.ErrStateNotFound
		}
		return nil, err
	}

	var raw oidc.PendingAuthState
	if err := json.Unmarshal(kve.Value(), &raw); err != nil {
		return nil, err
	}
	return &raw, nil
}

func (s *natsPASCache) Save(sess *oidc.PendingAuthState) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	_, err = s.kv.Put(context.Background(), sess.State, data)
	return err
}

func (s *natsPASCache) Delete(state string) error {
	return s.kv.Delete(context.Background(), state)
}

func (s *natsPASCache) Timeout() time.Duration {
	return s.cfg.Timeout
}

func init() {
	factory.Register(PendingAuthStateCacheTypeNats, New)
}
