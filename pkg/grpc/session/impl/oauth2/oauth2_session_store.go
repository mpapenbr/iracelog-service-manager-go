package oauth2

import (
	"time"

	"github.com/nats-io/nats.go"
	"golang.org/x/oauth2"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/oidc"
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
	ownCfg := &oauth2SessionStoreConfig{
		refreshThreshold: 10 * time.Second,
		storageCreator: func(storeCfg *oauth2SessionStoreConfig) (sessionStorage, error) {
			return newInMemoryStorage(cfg, storeCfg), nil
		},
	}
	for _, o := range specific {
		o(cfg, ownCfg)
	}
	storage, err := ownCfg.storageCreator(ownCfg)
	if err != nil {
		log.Error("error creating storage for oauth2 session store", log.ErrorField(err))
		return nil, err
	}
	ret := &oauth2SessionStore{
		cfg:     cfg,
		ownCfg:  ownCfg,
		log:     log.Default().Named("grpc.session.oauth2"),
		storage: storage,
	}
	ret.log.Debug("initialized oauth2 based session store",
		log.Duration("session-timeout", cfg.Timeout),
		log.Duration("refresh-threshold", ownCfg.refreshThreshold))

	return ret, nil
}

func NewSession(id string, opts ...SessionOption) session.Session {
	ret := &sessionImpl{
		raw: &sessionRawData{ID: id},
	}
	for _, o := range opts {
		o(ret)
	}
	return ret
}

type (
	Option        func(*session.Config, *oauth2SessionStoreConfig)
	SessionOption func(*sessionImpl)
	sessionImpl   struct {
		// lastAccessed time.Time
		tokenSource oauth2.TokenSource
		raw         *sessionRawData
	}
	sessionStorage interface {
		Get(id string) (*sessionImpl, error)
		Save(sess *sessionImpl) error
		Delete(id string) error
	}
	oauth2SessionStoreConfig struct {
		refreshThreshold time.Duration
		oidcParam        *oidc.OIDCParam
		storageCreator   func(cfg *oauth2SessionStoreConfig) (sessionStorage, error)
	}

	oauth2SessionStore struct {
		cfg     *session.Config
		ownCfg  *oauth2SessionStoreConfig
		storage sessionStorage
		log     *log.Logger
	}
)

var SessionTypeOAuth2 factory.SessionType = "oauth2"

func WithRefreshThreshold(d time.Duration) Option {
	return func(c *session.Config, s *oauth2SessionStoreConfig) {
		s.refreshThreshold = d
	}
}

func WithNATS(nc *nats.Conn) Option {
	return func(c *session.Config, s *oauth2SessionStoreConfig) {
		s.storageCreator = func(cfg *oauth2SessionStoreConfig) (sessionStorage, error) {
			return newNATSStorage(nc, c, cfg)
		}
	}
}

func WithOIDCParams(oidcParam *oidc.OIDCParam) Option {
	return func(c *session.Config, s *oauth2SessionStoreConfig) {
		s.oidcParam = oidcParam
	}
}

func WithSessionName(name string) SessionOption {
	return func(s *sessionImpl) {
		s.raw.Name = name
	}
}

func WithSessionRoles(roles []auth.Role) SessionOption {
	return func(s *sessionImpl) {
		s.raw.Roles = roles
	}
}

func WithSessionScopedRoles(scopedRoles []auth.ScopedRole) SessionOption {
	return func(s *sessionImpl) {
		s.raw.ScopedRoles = scopedRoles
	}
}

func WithSessionUserID(userID string) SessionOption {
	return func(s *sessionImpl) {
		s.raw.UserID = userID
	}
}

func WithSessionTenantID(tenantID uint32) SessionOption {
	return func(s *sessionImpl) {
		s.raw.TenantID = tenantID
	}
}

func WithSessionTokenSource(ts oauth2.TokenSource) SessionOption {
	return func(s *sessionImpl) {
		s.tokenSource = ts
	}
}

func WithSessionIDToken(idToken string) SessionOption {
	return func(s *sessionImpl) {
		s.raw.ExtraIDToken = idToken
	}
}

func GetIDToken(sess session.Session) string {
	if mySess, ok := sess.(*sessionImpl); ok {
		return mySess.raw.ExtraIDToken
	}
	return ""
}

func (s *oauth2SessionStore) Get(id string) (session.Session, error) {
	if sess, err := s.storage.Get(id); err == nil {
		if time.Since(sess.raw.LastAccessed) > s.cfg.Timeout {
			s.log.Debug("Session expired",
				log.String("id", id),
				log.Duration("sinceLast", time.Since(sess.raw.LastAccessed)),
				log.Duration("timeout", s.cfg.Timeout),
			)
			//nolint:errcheck // ignore error on delete
			s.storage.Delete(id)
			return nil, session.ErrSessionExpired
		} else {
			s.log.Debug("Session still valid",
				log.String("id", id),
				log.Duration("sinceLast", time.Since(sess.raw.LastAccessed)),
				log.Duration("timeout", s.cfg.Timeout),
			)
			sess.raw.LastAccessed = time.Now()
		}
		return sess, nil
	}

	return nil, session.ErrSessionNotFound
}

func (s *oauth2SessionStore) Save(sess session.Session) error {
	if mySession, ok := sess.(*sessionImpl); ok {
		token, err := mySession.tokenSource.Token()
		if err == nil {
			mySession.raw.Token = token
		}
		mySession.raw.LastAccessed = time.Now()
		// Note: the storage will take care of updating the token
		if sErr := s.storage.Save(mySession); sErr != nil {
			s.log.Warn("error saving session",
				log.String("id", mySession.raw.ID),
				log.ErrorField(sErr))
			return sErr
		}

		s.log.Debug("Saved session", log.String("id", mySession.raw.ID))

		return nil
	}
	return session.ErrInvalidSession
}

func (s *oauth2SessionStore) Delete(id string) error {
	return s.storage.Delete(id)
}

func (s *oauth2SessionStore) Timeout() time.Duration {
	return s.cfg.Timeout
}

func (s *sessionImpl) ID() string {
	return s.raw.ID
}

func (s *sessionImpl) Name() string {
	return s.raw.Name
}

func (s *sessionImpl) Roles() []auth.Role {
	return s.raw.Roles
}

func (s *sessionImpl) ScopedRoles() []auth.ScopedRole {
	return s.raw.ScopedRoles
}

func (s *sessionImpl) UserID() string {
	return s.raw.UserID
}

func (s *sessionImpl) TenantID() uint32 {
	return s.raw.TenantID
}

func init() {
	factory.Register(SessionTypeOAuth2, New)
}
