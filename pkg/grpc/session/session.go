package session

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
)

const SessionIDCookie = "iracelog_sessionid"

type (
	Session interface {
		ID() string
		Name() string
		Roles() []auth.Role
		ScopedRoles() []auth.ScopedRole
		UserID() string
		TenantID() uint32
	}
	SessionImpl struct {
		id          string
		name        string
		roles       []auth.Role
		scopedRoles []auth.ScopedRole
		userID      string
		tenantID    uint32
	}
	SessionStore interface {
		Get(id string) (Session, error)
		Save(s Session) error
		Delete(id string) error
		Timeout() time.Duration
	}
	sessionCtxKey struct{}
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
	ErrInvalidSession  = errors.New("invalid session for this store")
	sessionKey         = sessionCtxKey{}
)

var _ Session = (*SessionImpl)(nil)

func SessionFromContext(ctx context.Context) Session {
	if ctx == nil {
		return nil
	}
	if val, ok := ctx.Value(sessionKey).(Session); ok {
		return val
	}
	return nil
}

func AddSessionToContext(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionKey, session)
}

func CreateCookieForSession(session Session, timeout time.Duration) *http.Cookie {
	newCookie := &http.Cookie{
		Name:     SessionIDCookie,
		Value:    session.ID(),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,

		SameSite: http.SameSiteLaxMode,   // Or SameSiteNoneMode with Secure: true
		MaxAge:   int(timeout.Seconds()), // Set MaxAge to the session timeout in seconds
	}

	return newCookie
}

func (s *SessionImpl) ID() string {
	return s.id
}

func (s *SessionImpl) Name() string {
	return s.name
}

func (s *SessionImpl) Roles() []auth.Role {
	return s.roles
}

func (s *SessionImpl) ScopedRoles() []auth.ScopedRole {
	return s.scopedRoles
}

func (s *SessionImpl) UserID() string {
	return s.userID
}

func (s *SessionImpl) TenantID() uint32 {
	return s.tenantID
}
