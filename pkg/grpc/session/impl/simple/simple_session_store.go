package simple

import (
	"time"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session/factory"
)

func New(common []session.Option, specific []Option) (session.SessionStore, error) {
	cfg := &session.Config{
		Timeout: 30 * time.Second, // TODO: change to minutes after testing
	}
	for _, o := range common {
		o(cfg)
	}
	return &simpleSessionStore{
		cfg:      cfg,
		sessions: make(map[string]session.Session),
	}, nil
}

var SessionTypeSimple factory.SessionType = "simple"

type (
	Option             func(*simpleSessionStore)
	simpleSessionStore struct {
		cfg      *session.Config
		sessions map[string]session.Session
	}
)

func (s *simpleSessionStore) Create() (session.Session, error) {
	return nil, session.ErrSessionNotFound
}

func (s *simpleSessionStore) Get(id string) (session.Session, error) {
	if sess, ok := s.sessions[id]; ok {
		return sess, nil
	}
	return nil, session.ErrSessionNotFound
}

func (s *simpleSessionStore) Save(sess session.Session) error {
	s.sessions[sess.ID()] = sess
	return nil
}

func (s *simpleSessionStore) Delete(id string) error {
	delete(s.sessions, id)
	return nil
}

func (s *simpleSessionStore) Timeout() time.Duration {
	return s.cfg.Timeout
}

func (s *simpleSessionStore) CookieName() string {
	return s.cfg.CookieName
}

func init() {
	factory.Register(SessionTypeSimple, New)
}
