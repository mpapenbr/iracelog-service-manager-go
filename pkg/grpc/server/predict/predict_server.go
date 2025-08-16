package predict

import (
	"context"
	"time"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/predict/v1/predictv1connect"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	predictv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/predict/v1"
	"connectrpc.com/connect"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/racestints"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewServer(opts ...Option) *predictServer {
	ret := &predictServer{
		log: log.Default().Named("grpc.predict"),
	}
	for _, opt := range opts {
		opt(ret)
	}
	if ret.tracer == nil {
		ret.tracer = otel.Tracer("ism")
	}
	return ret
}

type Option func(*predictServer)

func WithRepositories(r api.Repositories) Option {
	return func(srv *predictServer) {
		srv.repos = r
	}
}

func WithEventLookup(lookup *utils.EventLookup) Option {
	return func(srv *predictServer) {
		srv.lookup = lookup
	}
}

func WithTracer(tracer trace.Tracer) Option {
	return func(srv *predictServer) {
		srv.tracer = tracer
	}
}

type predictServer struct {
	x.UnimplementedPredictServiceHandler
	lookup *utils.EventLookup
	repos  api.Repositories

	log    *log.Logger
	tracer trace.Tracer
}

//nolint:whitespace // by design
func (s *predictServer) GetPredictParam(
	ctx context.Context,
	req *connect.Request[predictv1.GetPredictParamRequest]) (
	*connect.Response[predictv1.GetPredictParamResponse], error,
) {
	s.log.Debug("GetPredictParam called",
		log.Any("arg", req.Msg),
		log.Int32("id", req.Msg.EventSelector.GetId()))
	var e *eventv1.Event
	var err error
	e, err = util.ResolveEvent(ctx, s.repos.Event(), req.Msg.EventSelector)
	if err != nil {
		s.log.Error("error resolving event",
			log.Any("selector", req.Msg.EventSelector),
			log.ErrorField(err))
		return nil, err
	}

	if req.Msg.StartSelector == nil {
		return nil, util.ErrInvalidStartSelector
	}

	var param *predictv1.PredictParam
	switch req.Msg.StartSelector.Arg.(type) {
	case *commonv1.StartSelector_RecordStamp:
		return nil, util.ErrUnsupportedStartSelector
	case *commonv1.StartSelector_SessionTime:
		param, err = GetPredictParam(
			ctx,
			s.repos,
			int(e.Id),
			time.Duration(req.Msg.StartSelector.GetSessionTime()*float32(time.Second)),
			req.Msg.CarNum,
			req.Msg.LaptimeSelector,
		)

	default:
		err = util.ErrUnsupportedStartSelector
	}
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&predictv1.GetPredictParamResponse{
		Param: param,
	}), nil
}

//nolint:whitespace // by design
func (s *predictServer) GetLivePredictParam(
	ctx context.Context,
	req *connect.Request[predictv1.GetLivePredictParamRequest]) (
	*connect.Response[predictv1.GetLivePredictParamResponse], error,
) {
	// get the epd
	var epd *utils.EventProcessingData
	var param *predictv1.PredictParam
	var err error
	epd, err = s.lookup.GetEvent(req.Msg.EventSelector)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	param, err = GetLivePredictParam(
		epd,
		req.Msg.CarNum,
		req.Msg.LaptimeSelector,
	)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&predictv1.GetLivePredictParamResponse{
		Param: param,
	}), nil
}

func (s *predictServer) PredictRace(ctx context.Context,
	req *connect.Request[predictv1.PredictRaceRequest]) (
	*connect.Response[predictv1.PredictRaceResponse], error,
) {
	// TODO: validate vital parameters
	result, err := racestints.Calc(req.Msg.Param)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&predictv1.PredictRaceResponse{
		Result: result,
	}), nil
}
