package simple

import (
	"time"

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
	return &simplePASCache{
		cfg:      cfg,
		pacCache: make(map[string]*oidc.PendingAuthState),
	}, nil
}

var PendingAuthStateCacheTypeSimple factory.CacheType = "simple"

type (
	Option         func(*simplePASCache)
	simplePASCache struct {
		cfg      *oidc.Config
		pacCache map[string]*oidc.PendingAuthState
	}
)

func (s *simplePASCache) Get(id string) (*oidc.PendingAuthState, error) {
	if sess, ok := s.pacCache[id]; ok {
		return sess, nil
	}
	return nil, oidc.ErrStateNotFound
}

func (s *simplePASCache) Save(sess *oidc.PendingAuthState) error {
	s.pacCache[sess.State] = sess
	return nil
}

func (s *simplePASCache) Delete(id string) error {
	delete(s.pacCache, id)
	return nil
}

func (s *simplePASCache) Timeout() time.Duration {
	return s.cfg.Timeout
}

func init() {
	factory.Register(PendingAuthStateCacheTypeSimple, New)
}
