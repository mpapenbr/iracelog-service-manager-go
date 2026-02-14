package impl

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/samber/lo"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/cache"
)

const (
	tokenHeader = "api-token"
)

type (
	authInterceptor struct {
		cfg          *auth.Config
		authProvider []auth.AuthenticationProvider
		l            *log.Logger
	}
)

func NewAuthInterceptor(opts ...auth.Option) connect.Interceptor {
	ret := &authInterceptor{
		cfg: &auth.Config{},
		l:   log.Default().Named("grpc.auth"),
	}
	for _, opt := range opts {
		opt(ret.cfg)
	}
	ret.authProvider = []auth.AuthenticationProvider{
		&apiKeyAuthenticator{
			adminToken:  ret.cfg.AdminToken,
			tenantCache: ret.cfg.TenantCache,
		},
		&sessionAuthenticator{tenantCache: ret.cfg.TenantCache},
		&anonymousAuthenticator{},
	}
	return ret
}

type (
	SimpleAuth struct {
		principal   auth.Principal
		roles       []auth.Role
		scopedRoles []auth.ScopedRole
	}
	SimplePrincipal struct {
		auth.Principal
		name string
	}
	TenantAuth struct {
		principal   auth.Principal
		roles       []auth.Role
		scopedRoles []auth.ScopedRole
		id          uint32 // this is the tenant id
	}
	SessionAuth struct {
		sessionID   string
		userID      string
		principal   auth.Principal
		roles       []auth.Role
		scopedRoles []auth.ScopedRole
		tenantID    uint32
	}
)

func (s *SimplePrincipal) Name() string {
	return s.name
}

func (s *SimpleAuth) Principal() auth.Principal {
	return s.principal
}

func (s *SimpleAuth) Roles() []auth.Role {
	return s.roles
}

func (s *SimpleAuth) ScopedRoles() []auth.ScopedRole {
	return s.scopedRoles
}

func (s *TenantAuth) Principal() auth.Principal {
	return s.principal
}

func (s *TenantAuth) Roles() []auth.Role {
	return s.roles
}

func (s *TenantAuth) ScopedRoles() []auth.ScopedRole {
	return s.scopedRoles
}

func (s *TenantAuth) GetTenantID() uint32 {
	return s.id
}

func (s *SessionAuth) Principal() auth.Principal {
	return s.principal
}

func (s *SessionAuth) Roles() []auth.Role {
	return s.roles
}

func (s *SessionAuth) ScopedRoles() []auth.ScopedRole {
	return s.scopedRoles
}

func (s *SessionAuth) GetTenantID() uint32 {
	return s.tenantID
}

func (s *SessionAuth) GetUserID() string {
	return s.userID
}

func (s *SessionAuth) GetSessionID() string {
	return s.sessionID
}

func (s *SessionAuth) CombinedRoles() []auth.Role {
	work := lo.Map(s.scopedRoles,
		func(item auth.ScopedRole, _ int) auth.Role { return item.Role })
	return append(s.roles, work...)
}

func NewSimplePrincipal(name string) *SimplePrincipal {
	return &SimplePrincipal{name: name}
}

// compile time check if everything implements the interface
var (
	_ auth.Authentication       = (*SimpleAuth)(nil)
	_ auth.TenantAuthentication = (*TenantAuth)(nil)
	_ auth.TenantAuthentication = (*SessionAuth)(nil)
)

var anon = &SimpleAuth{principal: &SimplePrincipal{name: "anon"}, roles: []auth.Role{}}

//nolint:whitespace // can't make both editor and linter happy
func (i *authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return connect.UnaryFunc(func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		return next(i.handleAuth(ctx, req.Header()), req)
	})
}

//
//nolint:whitespace // editor/linter issue
func (i *authInterceptor) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return next
}

//
//nolint:lll,whitespace // better readability
func (i *authInterceptor) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return connect.StreamingHandlerFunc(func(
		ctx context.Context,
		conn connect.StreamingHandlerConn,
	) error {
		return next(i.handleAuth(ctx, conn.RequestHeader()), conn)
	})
}

//nolint:lll // better readability
func (i *authInterceptor) handleAuth(ctx context.Context, h http.Header) context.Context {
	for _, p := range i.authProvider {
		a, err := p.Authenticate(ctx, h)
		if a != nil {
			return auth.AddAuthToContext(ctx, a)
		}
		if err != nil {
			i.l.Error("error authenticating", log.ErrorField(err))
		}
	}
	// if no auth found, continue with current context
	return ctx
}

type (
	anonymousAuthenticator struct{}
	apiKeyAuthenticator    struct {
		adminToken  string
		tenantCache cache.Cache[string, model.Tenant]
	}
	sessionAuthenticator struct {
		tenantCache cache.Cache[string, model.Tenant]
	}
)

//nolint:whitespace // editor/linter issue
func (a *anonymousAuthenticator) Authenticate(
	ctx context.Context,
	h http.Header,
) (auth.Authentication, error) {
	return anon, nil
}

//nolint:lll,whitespace // editor/linter issue
func (a *apiKeyAuthenticator) Authenticate(
	ctx context.Context,
	h http.Header,
) (auth.Authentication, error) {
	if h.Get(tokenHeader) == "" {
		return nil, nil
	}
	if h.Get(tokenHeader) == a.adminToken {
		return &SimpleAuth{
			principal: &SimplePrincipal{name: "admin"},
			roles:     []auth.Role{auth.RoleAdmin},
		}, nil
	}

	if t, err := a.tenantCache.Get(ctx, utils.HashAPIKey(h.Get(tokenHeader))); err == nil &&
		t.Tenant.IsActive {

		return &TenantAuth{
			principal: &SimplePrincipal{name: t.Tenant.Name},
			roles:     []auth.Role{},
			scopedRoles: []auth.ScopedRole{
				{Role: auth.RoleRaceDataProvider, Scopes: []string{fmt.Sprintf("%d", t.ID)}},
			},
			id: t.ID,
		}, nil
	} else {
		return nil, err
	}
}

//nolint:lll,whitespace // editor/linter issue
func (a *sessionAuthenticator) Authenticate(
	ctx context.Context,
	h http.Header,
) (auth.Authentication, error) {
	if sInfo := session.SessionFromContext(ctx); sInfo != nil {
		log.Debug("found session in context", log.String("session_id", sInfo.ID()))
		return &SessionAuth{
			sessionID:   sInfo.ID(),
			userID:      sInfo.UserID(),
			principal:   &SimplePrincipal{name: sInfo.Name()},
			roles:       sInfo.Roles(),
			scopedRoles: sInfo.ScopedRoles(),
		}, nil
	}

	return nil, nil
}
