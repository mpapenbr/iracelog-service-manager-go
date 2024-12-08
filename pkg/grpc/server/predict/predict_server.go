package predict

import (
	"context"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/predict/v1/predictv1connect"
	predictv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/predict/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

func NewServer(opts ...Option) *predictServer {
	ret := &predictServer{
		log: log.Default().Named("grpc.predict"),
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type Option func(*predictServer)

func WithPool(p *pgxpool.Pool) Option {
	return func(srv *predictServer) {
		srv.pool = p
	}
}

type predictServer struct {
	x.UnimplementedPredictServiceHandler

	pool *pgxpool.Pool
	log  *log.Logger
}

//nolint:whitespace // by design
func (s *predictServer) GetPredictParam(
	ctx context.Context,
	req *connect.Request[predictv1.GetPredictParamRequest]) (
	*connect.Response[predictv1.GetPredictParamResponse], error,
) {
	return nil, nil
}

//nolint:whitespace // by design
func (s *predictServer) GetLivePredictParam(
	ctx context.Context,
	req *connect.Request[predictv1.GetLivePredictParamRequest]) (
	*connect.Response[predictv1.GetLivePredictParamResponse], error,
) {
	return nil, nil
}
