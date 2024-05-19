package auth

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
)

const (
	tokenHeader = "api-token"
)

type (
	authInterceptor struct {
		adminToken    string
		providerToken string
	}
	Option func(*authInterceptor)
)

func NewAuthInterceptor(opts ...Option) *authInterceptor {
	ret := &authInterceptor{}
	for _, opt := range opts {
		opt(ret)
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

func FromContext(ctx *context.Context) *Authentication {
	if ctx == nil {
		return nil
	}
	if val, ok := (*ctx).Value(myCtxTypeKey(0)).(*AuthHolder); ok {
		return &val.auth
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
	if h.Get(tokenHeader) == "" {
		// no token, no auth
		return context.WithValue(ctx, myCtxTypeKey(0), &AuthHolder{auth: anon})
	}
	if h.Get(tokenHeader) == i.adminToken {
		return context.WithValue(ctx, myCtxTypeKey(0), &AuthHolder{auth: &SimpleAuth{
			principal: &SimplePrincipal{name: "admin"},
			roles:     []Role{RoleAdmin, RoleProvider},
		}})
	}

	if h.Get(tokenHeader) == i.providerToken {
		return context.WithValue(ctx, myCtxTypeKey(0), &AuthHolder{auth: &SimpleAuth{
			principal: &SimplePrincipal{name: "provider"},
			roles:     []Role{RoleProvider},
		}})
	}

	return context.WithValue(ctx, myCtxTypeKey(0), &AuthHolder{auth: anon})
}
