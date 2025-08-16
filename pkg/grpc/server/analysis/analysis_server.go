package analysis

import (
	"context"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/analysis/v1/analysisv1connect"
	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	"connectrpc.com/connect"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
)

func NewServer(opts ...Option) *analysisServer {
	ret := &analysisServer{
		log: log.Default().Named("grpc.analysis"),
	}
	for _, opt := range opts {
		opt(ret)
	}
	if ret.tracer == nil {
		ret.tracer = otel.Tracer("ism")
	}
	return ret
}

type Option func(*analysisServer)

func WithRepositories(r api.Repositories) Option {
	return func(srv *analysisServer) {
		srv.repos = r
	}
}

func WithTransactionManager(txMgr api.TransactionManager) Option {
	return func(srv *analysisServer) {
		srv.txMgr = txMgr
	}
}

func WithPermissionEvaluator(pe permission.PermissionEvaluator) Option {
	return func(srv *analysisServer) {
		srv.pe = pe
	}
}

func WithTracer(tracer trace.Tracer) Option {
	return func(srv *analysisServer) {
		srv.tracer = tracer
	}
}

type analysisServer struct {
	x.UnimplementedAnalysisServiceHandler
	pe     permission.PermissionEvaluator
	repos  api.Repositories
	txMgr  api.TransactionManager
	log    *log.Logger
	tracer trace.Tracer
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

	e, err = util.ResolveEvent(ctx, s.repos.Event(), req.Msg.EventSelector)
	if err != nil {
		s.log.Error("error resolving event",
			log.Any("selector", req.Msg.EventSelector),
			log.ErrorField(err))
		return nil, err
	}
	analysisData, err = s.repos.Analysis().LoadByEventID(ctx, int(e.GetId()))
	if err != nil {
		return nil, err
	}
	snapshots, err = s.repos.Speedmap().LoadSnapshots(ctx, int(e.GetId()), 300)
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

	e, err = util.ResolveEvent(ctx, s.repos.Event(), req.Msg.EventSelector)
	if err != nil {
		s.log.Error("error resolving event",
			log.Any("selector", req.Msg.EventSelector),
			log.ErrorField(err))
		return nil, err
	}
	analysisData, err = s.repos.Analysis().LoadByEventID(ctx, int(e.GetId()))
	if err != nil {
		return nil, err
	}
	var recompAnalysisData *analysisv1.Analysis
	recompAnalysisData, err = RecomputeAnalysis(ctx, s.repos, e)
	if err != nil {
		s.log.Error("error recomputing analysis",
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
		func(ctx context.Context) error {
			return s.repos.Analysis().Upsert(
				context.Background(),
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
	storeFunc func(ctx context.Context) error,
) error {
	return s.txMgr.RunInTx(ctx, func(ctx context.Context) error {
		return storeFunc(ctx)
	})
}
