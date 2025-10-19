package auth

import (
	"context"
	"errors"
	"net/http"
)

type (
	Role       string
	ScopedRole struct {
		Role   Role
		Scopes []string
	}
	Principal interface {
		Name() string
	}
	Authentication interface {
		Principal() Principal
		Roles() []Role
		ScopedRoles() []ScopedRole
	}
	TenantAuthentication interface {
		Authentication
		GetTenantID() uint32
	}
	AuthenticationProvider interface {
		Authenticate(ctx context.Context, h http.Header) (Authentication, error)
	}
	myCtxTypeKey struct{}
)

const (
	RoleAdmin            Role = "admin"
	RoleRaceDataProvider Role = "racedata-provider"
	RoleEditor           Role = "editor"
	RoleTenantManager    Role = "tenant-manager"
	RoleTrackManager     Role = "track-manager"
)

var (
	GlobalRoles = []Role{
		RoleAdmin,
		RoleTenantManager,
		RoleTrackManager,
	}
	ScopedRoles = []Role{
		RoleRaceDataProvider,
		RoleEditor,
	}
	ctxKey = myCtxTypeKey{}
)

var ErrPermissionDenied = errors.New("permission denied")

func FromContext(ctx *context.Context) Authentication {
	if ctx == nil {
		return nil
	}
	if val, ok := (*ctx).Value(ctxKey).(Authentication); ok {
		return val
	}
	return nil
}

func AddAuthToContext(ctx context.Context, auth Authentication) context.Context {
	return context.WithValue(ctx, ctxKey, auth)
}
