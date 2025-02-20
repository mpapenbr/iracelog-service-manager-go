package auth

import (
	"context"
	"errors"
	"net/http"
)

type (
	Role string
)

const (
	RoleAdmin            Role = "admin"
	RoleRaceDataProvider Role = "racedata-provider"
	RoleTenantManager    Role = "tenant-manager"
	RoleTrackManager     Role = "track-manager"
)

var ErrPermissionDenied = errors.New("permission denied")

type (
	Principal interface {
		Name() string
	}
	Authentication interface {
		Principal() Principal
		Roles() []Role
	}
	TenantAuthentication interface {
		Authentication
		GetId() uint32 // returns the tenant id
	}
	AuthenticationProvider interface {
		Authenticate(ctx context.Context, h http.Header) (Authentication, error)
	}
)
