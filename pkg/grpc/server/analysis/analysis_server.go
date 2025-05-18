package analysis

import (
	"context"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/analysis/v1/analysisv1connect"
	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	aProto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/analysis/proto"
	smProto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/speedmap/proto"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
)

func NewServer(opts ...Option) *analysisServer {
	ret := &analysisServer{
		log: log.Default().Named("grpc.analysis"),
	}
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
	log  *log.Logger
}

//nolint:whitespace // can't make both editor and linter happy
func (s *analysisServer) GetAnalysis(
	ctx context.Context, req *connect.Request[analysisv1.GetAnalysisRequest],
) (*connect.Response[analysisv1.GetAnalysisResponse], error) {
	s.log.Debug("GetAnalysis called",
		log.Any("arg", req.Msg),
		log.Int32("id", req.Msg.EventSelector.GetId()))
	var analysisData *analysisv1.Analysis
	var snapshots []*analysisv1.SnapshotData
	var err error
	var e *eventv1.Event

	e, err = util.ResolveEvent(ctx, s.pool, req.Msg.EventSelector)
	if err != nil {
		s.log.Error("error resolving event",
			log.Any("selector", req.Msg.EventSelector),
			log.ErrorField(err))
		return nil, err
	}
	analysisData, err = aProto.LoadByEventID(ctx, s.pool, int(e.GetId()))
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

//nolint:whitespace,funlen // by design
func (s *analysisServer) ComputeAnalysis(
	ctx context.Context, req *connect.Request[analysisv1.ComputeAnalysisRequest],
) (*connect.Response[analysisv1.ComputeAnalysisResponse], error) {
	s.log.Debug("ComputeAnalysis called",
		log.Any("arg", req.Msg),
		log.Int32("id", req.Msg.EventSelector.GetId()))
	a := auth.FromContext(&ctx)
	if !s.pe.HasPermission(a, permission.PermissionUpdateEvent) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}
	var analysisData *analysisv1.Analysis

	var err error
	var e *eventv1.Event

	e, err = util.ResolveEvent(ctx, s.pool, req.Msg.EventSelector)
	if err != nil {
		s.log.Error("error resolving event",
			log.Any("selector", req.Msg.EventSelector),
			log.ErrorField(err))
		return nil, err
	}
	analysisData, err = aProto.LoadByEventID(ctx, s.pool, int(e.GetId()))
	if err != nil {
		return nil, err
	}
	var recompAnalysisData *analysisv1.Analysis
	recompAnalysisData, err = RecomputeAnalysis(ctx, s.pool, e)
	if err != nil {
		s.log.Error("error recomputeing analysis",
			log.Any("selector", req.Msg.EventSelector),
			log.ErrorField(err))
		return nil, err
	}
	// lets see what we need to update
	//nolint:exhaustive // by design
	switch req.Msg.Component {
	case analysisv1.AnalysisComponent_ANALYSIS_COMPONENT_RACEGRAPH:
		analysisData.RaceGraph = recompAnalysisData.RaceGraph
	case analysisv1.AnalysisComponent_ANALYSIS_COMPONENT_ALL:
		analysisData.RaceGraph = recompAnalysisData.RaceGraph
	}
	// finally store the data if requested
	if req.Msg.PersistMode == analysisv1.AnalysisPersistMode_ANALYSIS_PERSIST_MODE_ON {
		log.Debug("storing analysis data",
			log.Any("selector", req.Msg.EventSelector))
		s.storeAnalysisData(int(e.GetId()), analysisData)
	} else {
		log.Debug("not storing analysis data",
			log.Any("selector", req.Msg.EventSelector),
			log.String("persist_mode", req.Msg.PersistMode.String()))
	}

	return connect.NewResponse(&analysisv1.ComputeAnalysisResponse{
		Analysis: analysisData,
	}), nil
}

//nolint:whitespace,dupl // false positive
func (s *analysisServer) storeAnalysisData(
	eventID int,
	data *analysisv1.Analysis,
) {
	if err := s.storeData(
		context.Background(),
		func(ctx context.Context, tx pgx.Tx) error {
			return aProto.Upsert(
				context.Background(),
				s.pool,
				eventID,
				data)
		}); err != nil {
		s.log.Error("error storing analysis data", log.ErrorField(err))
	}
}

// helper function to store data in the database within a transaction
// function evalates epd.RecordingMode to determine if data should be stored
//
//nolint:whitespace // can't make both editor and linter happy
func (s *analysisServer) storeData(
	ctx context.Context,
	storeFunc func(ctx context.Context, tx pgx.Tx) error,
) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := storeFunc(ctx, tx); err != nil {
			return err
		}
		return nil
	})
}
