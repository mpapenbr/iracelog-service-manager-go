package analysis

import (
	"context"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/analysis/v1/analysisv1connect"
	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	aProto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/analysis/proto"
	smProto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/speedmap/proto"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
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
	var analysisData *analysisv1.Analysis
	var snapshots []*analysisv1.SnapshotData
	var err error
	var e *eventv1.Event

	e, err = util.ResolveEvent(ctx, s.pool, req.Msg.EventSelector)
	if err != nil {
		log.Error("error resolving event",
			log.Any("selector", req.Msg.EventSelector),
			log.ErrorField(err))
		return nil, err
	}
	analysisData, err = aProto.LoadByEventId(ctx, s.pool, int(e.GetId()))
	if err != nil {
		return nil, err
	}
	snapshots, err = smProto.LoadSnapshots(ctx, s.pool, int(e.GetId()), 300)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&analysisv1.GetAnalysisResponse{
		Analysis:  analysisData,
		Snapshots: snapshots,
	}), nil
}
