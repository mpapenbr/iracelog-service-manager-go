package settings

import (
	"context"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/settings/v1/settingsv1connect"
	settingsv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/settings/v1"
	"connectrpc.com/connect"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

func NewServer(opts ...Option) *settingsServer {
	ret := &settingsServer{
		log: log.Default().Named("grpc.settings"),
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
	Option func(*settingsServer)
)

func WithTracer(tracer trace.Tracer) Option {
	return func(srv *settingsServer) {
		srv.tracer = tracer
	}
}

func WithSupportsLogin(supportsLogin bool) Option {
	return func(srv *settingsServer) {
		srv.supportsLogin = supportsLogin
	}
}

type (
	settingsServer struct {
		x.UnimplementedSettingsServiceHandler
		log           *log.Logger
		tracer        trace.Tracer
		supportsLogin bool
	}
)

//nolint:whitespace // can't make both editor and linter happy
func (s *settingsServer) GetServerSettings(
	ctx context.Context,
	req *connect.Request[settingsv1.GetServerSettingsRequest],
) (*connect.Response[settingsv1.GetServerSettingsResponse], error) {
	s.log.Debug("GetServerSettings called")

	resp := connect.NewResponse(&settingsv1.GetServerSettingsResponse{
		SupportsLogin: s.supportsLogin,
	})

	return resp, nil
}
