package analysis

import (
	"context"
	"errors"

	x "buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/analysis/v1/analysisv1connect"
	analysisv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/analysis/v1"
	commonv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/common/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	aProto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/analysis/proto"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewServer(opts ...Option) *analysisServer {
	ret := &analysisServer{}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type Option func(*analysisServer)

func WithPool(p *pgxpool.Pool) Option {
	return func(srv *analysisServer) {
		srv.pool = p
	}
}

func WithPermissionEvaluator(pe permission.PermissionEvaluator) Option {
	return func(srv *analysisServer) {
		srv.pe = pe
	}
}

type analysisServer struct {
	x.UnimplementedAnalysisServiceHandler
	pe   permission.PermissionEvaluator
	pool *pgxpool.Pool
}

//nolint:whitespace // can't make both editor and linter happy
func (s *analysisServer) GetAnalysis(
	ctx context.Context, req *connect.Request[analysisv1.GetAnalysisRequest],
) (*connect.Response[analysisv1.GetAnalysisResponse], error) {
	log.Debug("GetAnalysis called",
		log.Any("arg", req.Msg),
		log.Int32("id", req.Msg.EventSelector.GetId()))
	var data *analysisv1.Analysis
	var err error
	switch req.Msg.EventSelector.Arg.(type) {
	case *commonv1.EventSelector_Id:
		data, err = aProto.LoadByEventId(ctx, s.pool, int(req.Msg.EventSelector.GetId()))
	case *commonv1.EventSelector_Key:
		data, err = aProto.LoadByEventKey(context.Background(), s.pool,
			req.Msg.EventSelector.GetKey())
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, utils.ErrEventNotFound)
		}
		return nil, err
	}
	return connect.NewResponse(&analysisv1.GetAnalysisResponse{
		Analysis: data,
	}), nil
}
