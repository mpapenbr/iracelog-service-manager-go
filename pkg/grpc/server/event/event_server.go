package event

import (
	"context"
	"database/sql"
	"errors"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/event/v1/eventv1connect"
	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	carv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/car/v1"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"connectrpc.com/connect"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
	eventservice "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/service/event"
)

func NewServer(opts ...Option) *eventsServer {
	ret := &eventsServer{
		log: log.Default().Named("grpc.event"),
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
	Option func(*eventsServer)
)

func WithRepositories(r api.Repositories) Option {
	return func(srv *eventsServer) {
		srv.repos = r
	}
}

func WithTxManager(txMgr api.TransactionManager) Option {
	return func(srv *eventsServer) {
		srv.txMgr = txMgr
	}
}

func WithEventService(s *eventservice.EventService) Option {
	return func(srv *eventsServer) {
		srv.service = s
	}
}

func WithPermissionEvaluator(pe permission.PermissionEvaluator) Option {
	return func(srv *eventsServer) {
		srv.pe = pe
	}
}

func WithTracer(tracer trace.Tracer) Option {
	return func(srv *eventsServer) {
		srv.tracer = tracer
	}
}

type eventsServer struct {
	x.UnimplementedEventServiceHandler
	service *eventservice.EventService
	pe      permission.PermissionEvaluator
	log     *log.Logger
	repos   api.Repositories
	txMgr   api.TransactionManager
	tracer  trace.Tracer
}

//nolint:whitespace // can't make both editor and linter happy
func (s *eventsServer) GetEvents(
	ctx context.Context,
	req *connect.Request[eventv1.GetEventsRequest],
	stream *connect.ServerStream[eventv1.GetEventsResponse],
) error {
	var tenantID *uint32 = nil
	if t, err := util.ResolveTenant(
		ctx,
		s.repos.Tenant(),
		req.Msg.TenantSelector); err == nil {
		if t != nil {
			tenantID = &t.ID
		}
	} else {
		return err
	}
	data, err := s.repos.Event().LoadAll(ctx, tenantID)
	if err != nil {
		return err
	}
	for i := range data {
		if err := stream.Send(
			&eventv1.GetEventsResponse{Event: data[i]}); err != nil {
			s.log.Error("Error sending event", log.ErrorField(err))
			return err
		}
	}
	return nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *eventsServer) GetLatestEvents(
	ctx context.Context,
	req *connect.Request[eventv1.GetLatestEventsRequest],
) (*connect.Response[eventv1.GetLatestEventsResponse], error) {
	var tenantID *uint32 = nil
	if t, err := util.ResolveTenant(
		ctx,
		s.repos.Tenant(),
		req.Msg.TenantSelector); err == nil {
		if t != nil {
			tenantID = &t.ID
		}
	} else {
		return nil, err
	}
	data, err := s.repos.Event().LoadAll(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&eventv1.GetLatestEventsResponse{Events: data}), nil
}

//nolint:whitespace,funlen // can't make both editor and linter happy
func (s *eventsServer) GetEvent(
	ctx context.Context, req *connect.Request[eventv1.GetEventRequest],
) (*connect.Response[eventv1.GetEventResponse], error) {
	s.log.Debug("GetEvent called",
		log.Any("arg", req.Msg),
		log.Int32("id", req.Msg.EventSelector.GetId()))
	var e *eventv1.Event
	var a *analysisv1.Analysis

	var err error
	e, err = util.ResolveEvent(ctx, s.repos.Event(), req.Msg.EventSelector)
	if err != nil {
		s.log.Error("error resolving event",
			log.Any("selector", req.Msg.EventSelector),
			log.ErrorField(err))
		return nil, err
	}

	a, err = s.repos.Analysis().LoadByEventID(ctx, int(e.Id))
	if err != nil {
		s.log.Error("error loading event",
			log.Uint32("eventId", e.Id),
			log.ErrorField(err))
		return nil, err
	}

	t, err := s.repos.Track().LoadByID(ctx, int(e.TrackId))
	if err != nil {
		s.log.Error("error loading track",
			log.Uint32("eventId", e.Id),
			log.Uint32("trackId", e.TrackId),
			log.ErrorField(err))
		return nil, err
	}
	cd, err := s.repos.CarProto().LoadLatest(ctx, int(e.Id))
	if err != nil {
		s.log.Error("error loading car proto data",
			log.Uint32("eventId", e.Id),
			log.ErrorField(err))
		return nil, err
	}
	sd, err := s.repos.Racestate().LoadLatest(ctx, int(e.Id))
	if err != nil {
		s.log.Error("error loading race proto data",
			log.Uint32("eventId", e.Id),
			log.ErrorField(err))
		return nil, err
	}
	m, err := s.repos.Racestate().CollectMessages(ctx, int(e.Id))
	if err != nil {
		s.log.Error("error collecting messages",
			log.Uint32("eventId", e.Id),
			log.ErrorField(err))
		return nil, err
	}
	s.log.Debug("message collected", log.Int("num", len(m)))
	sm, err := s.repos.Speedmap().LoadLatest(ctx, int(e.Id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sm = &racestatev1.PublishSpeedmapRequest{}
		} else {
			s.log.Error("error loading speedmap proto data",
				log.Uint32("eventId", e.Id),
				log.ErrorField(err))
			return nil, err
		}
	}

	snapshots, err := s.repos.Speedmap().LoadSnapshots(ctx, int(e.Id), 120)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&eventv1.GetEventResponse{
		Event: e, Track: t, Analysis: a,
		Car: &carv1.CarContainer{
			Cars:           cd.Cars,
			Entries:        cd.Entries,
			CarClasses:     cd.CarClasses,
			CurrentDrivers: cd.CurrentDrivers,
		},
		State: &racestatev1.StateContainer{
			Session:  sd.Session,
			Cars:     sd.Cars,
			Messages: m,
		},
		Speedmap:  sm.Speedmap,
		Snapshots: snapshots,
	}), nil
}

//nolint:whitespace,gocritic,funlen // can't make both editor and linter happy
func (s *eventsServer) DeleteEvent(
	ctx context.Context, req *connect.Request[eventv1.DeleteEventRequest],
) (*connect.Response[eventv1.DeleteEventResponse], error) {
	a := auth.FromContext(&ctx)

	s.log.Debug("DeleteEvent called",
		log.Any("arg", req.Msg),
		log.Int32("id", req.Msg.EventSelector.GetId()))

	data, err := s.validateEventAccess(
		ctx,
		a,
		permission.PermissionDeleteEvent,
		req.Msg.EventSelector)
	if err != nil {
		return nil, err
	}

	err = s.service.DeleteEvent(ctx, int(data.Id))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&eventv1.DeleteEventResponse{}), nil
}

//nolint:whitespace,gocritic,funlen // can't make both editor and linter happy
func (s *eventsServer) UpdateEvent(
	ctx context.Context, req *connect.Request[eventv1.UpdateEventRequest],
) (*connect.Response[eventv1.UpdateEventResponse], error) {
	a := auth.FromContext(&ctx)
	s.log.Debug("UpdateEvent called",
		log.Any("arg", req.Msg),
		log.Int32("id", req.Msg.EventSelector.GetId()))
	data, err := s.validateEventAccess(
		ctx,
		a,
		permission.PermissionUpdateEvent,
		req.Msg.EventSelector)
	if err != nil {
		return nil, err
	}

	if data, err = s.service.UpdateEvent(ctx, int(data.Id), req.Msg); err == nil {
		return connect.NewResponse(&eventv1.UpdateEventResponse{Event: data}), nil
	}
	return nil, err
}

func (s *eventsServer) validateEventAccess(
	ctx context.Context,
	a auth.Authentication,
	perm permission.Permission,
	eventSel *commonv1.EventSelector,
) (*eventv1.Event, error) {
	// get the event
	data, err := util.ResolveEvent(ctx, s.repos.Event(), eventSel)
	if err != nil {
		ta, ok := a.(auth.TenantAuthentication)
		if ok && s.pe.HasTenantPermission(a, perm, ta.GetTenantID()) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		} else {
			return nil, connect.NewError(
				connect.CodePermissionDenied,
				auth.ErrPermissionDenied)
		}
	}
	t, err := s.repos.Tenant().LoadByEventID(ctx, int(data.Id))
	if err != nil {
		return nil, err
	}
	if !s.pe.HasTenantPermission(a,
		perm,
		t.ID) {

		return nil, connect.NewError(
			connect.CodePermissionDenied,
			auth.ErrPermissionDenied)
	}
	return data, nil
}
