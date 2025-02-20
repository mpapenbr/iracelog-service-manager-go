package auth

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/cache"
)

const (
	tokenHeader = "api-token"
)

type (
	authInterceptor struct {
		adminToken    string
		providerToken string
		tenantCache   cache.Cache[string, model.Tenant]
		authProvider  []AuthenticationProvider
		l             *log.Logger
	}
	Option func(*authInterceptor)
)

func NewAuthInterceptor(opts ...Option) connect.Interceptor {
	ret := &authInterceptor{
		l: log.Default().Named("grpc.auth"),
	}
	for _, opt := range opts {
		opt(ret)
	}
	ret.authProvider = []AuthenticationProvider{
		&apiKeyAuthenticator{adminToken: ret.adminToken, tenantCache: ret.tenantCache},
		&anonymousAuthenticator{},
	}
	return ret
}

func WithAuthToken(token string) Option {
	return func(srv *authInterceptor) {
		srv.adminToken = token
	}
}

func WithProviderToken(token string) Option {
	return func(srv *authInterceptor) {
		srv.providerToken = token
	}
}

func WithTenantCache(arg cache.Cache[string, model.Tenant]) Option {
	return func(srv *authInterceptor) {
		srv.tenantCache = arg
	}
}

type AuthHolder struct {
	auth Authentication
}

type SimpleAuth struct {
	Authentication
	principal Principal
	roles     []Role
}
type SimplePrincipal struct {
	Principal
	name string
}

func (s *SimplePrincipal) Name() string {
	return s.name
}

func (s *SimpleAuth) Principal() Principal {
	return s.principal
}

func (s *SimpleAuth) Roles() []Role {
	return s.roles
}

var anon = &SimpleAuth{principal: &SimplePrincipal{name: "anon"}, roles: []Role{}}

type myCtxTypeKey int

func FromContext(ctx *context.Context) Authentication {
	if ctx == nil {
		return nil
	}
	if val, ok := (*ctx).Value(myCtxTypeKey(0)).(*AuthHolder); ok {
		return val.auth
	}
	return nil
}

//nolint:whitespace // can't make both editor and linter happy
func (i *authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return connect.UnaryFunc(func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		return next(i.handleAuth(ctx, req.Header()), req)
	})
}

//nolint:lll // better readability
func (i *authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

//nolint:lll,whitespace // better readability
func (i *authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
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
			return context.WithValue(ctx, myCtxTypeKey(0), &AuthHolder{auth: a})
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
)

//nolint:whitespace // editor/linter issue
func (a *anonymousAuthenticator) Authenticate(
	ctx context.Context,
	h http.Header,
) (Authentication, error) {
	return anon, nil
}

//nolint:lll,whitespace // editor/linter issue
func (a *apiKeyAuthenticator) Authenticate(
	ctx context.Context,
	h http.Header,
) (Authentication, error) {
	if h.Get(tokenHeader) == "" {
		return nil, nil
	}
	if h.Get(tokenHeader) == a.adminToken {
		return &SimpleAuth{
			principal: &SimplePrincipal{name: "admin"},
			roles:     []Role{RoleAdmin},
		}, nil
	}

	if t, err := a.tenantCache.Get(ctx, utils.HashApiKey(h.Get(tokenHeader))); err == nil &&
		t.Tenant.IsActive {

		return &SimpleAuth{
			principal: &SimplePrincipal{name: t.Tenant.Name},
			roles:     []Role{RoleRaceDataProvider},
		}, nil
	} else {
		return nil, err
	}
}
