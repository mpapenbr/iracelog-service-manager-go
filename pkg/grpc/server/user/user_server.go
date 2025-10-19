package user

import (
	"context"
	"errors"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/user/v1/userv1connect"
	userv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/user/v1"
	"connectrpc.com/connect"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	authImpl "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth/impl"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session"
)

func NewServer(opts ...Option) *userServer {
	ret := &userServer{
		log: log.Default().Named("grpc.user"),
	}
	for _, opt := range opts {
		opt(ret)
	}

	if ret.tracer == nil {
		ret.tracer = otel.Tracer("ism")
	}
	return ret
}

type (
	Option func(*userServer)
)

func WithTracer(tracer trace.Tracer) Option {
	return func(srv *userServer) {
		srv.tracer = tracer
	}
}

func WithSessionStore(sessionStore session.SessionStore) Option {
	return func(srv *userServer) {
		srv.sessionStore = sessionStore
	}
}

var ErrUserNotFound = errors.New("user not found")

type (
	userServer struct {
		x.UnimplementedUserServiceHandler
		log    *log.Logger
		tracer trace.Tracer

		sessionStore session.SessionStore
	}
)

//nolint:whitespace // can't make both editor and linter happy
func (s *userServer) GetUserInfo(
	ctx context.Context,
	req *connect.Request[userv1.GetUserInfoRequest],
) (*connect.Response[userv1.GetUserInfoResponse], error) {
	s.log.Debug("GetUserInfo called")
	a := auth.FromContext(&ctx)
	if sAuth, ok := a.(*authImpl.SessionAuth); ok {
		outboundRoles := lo.Uniq(lo.Map(sAuth.Roles(),
			func(item auth.Role, _ int) string { return string(item) },
		))

		scopedRoles := lo.Map(sAuth.ScopedRoles(),
			func(sr auth.ScopedRole, _ int) *userv1.ScopedRole {
				return &userv1.ScopedRole{
					Role:   string(sr.Role),
					Scopes: sr.Scopes,
				}
			})
		userInfo := &userv1.UserInfo{
			UserId:      sAuth.GetUserID(),
			Username:    a.Principal().Name(),
			Roles:       outboundRoles,
			IsAdmin:     lo.Contains(sAuth.Roles(), auth.RoleAdmin),
			IsEditor:    lo.Contains(sAuth.CombinedRoles(), auth.RoleEditor),
			ScopedRoles: scopedRoles,
		}
		resp := connect.NewResponse(&userv1.GetUserInfoResponse{
			UserInfo: userInfo,
		})
		return resp, nil

	} else {
		// a is not of type *auth.SessionAuth
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("no auth info in context"))
	}
}
