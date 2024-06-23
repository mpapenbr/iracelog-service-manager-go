package event

import (
	"context"
	"errors"
	"time"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/event/v1/eventv1connect"
	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	carv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/car/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	aProto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/analysis/proto"
	cProto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/car/proto"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/event"
	rProto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/racestate"
	smProto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/speedmap/proto"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/track"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
	eventservice "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/service/event"
)

func NewServer(opts ...Option) *eventsServer {
	ret := &eventsServer{}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type Option func(*eventsServer)

func WithPool(p *pgxpool.Pool) Option {
	return func(srv *eventsServer) {
		srv.pool = p
		srv.service = eventservice.NewEventService(p)
	}
}

func WithPermissionEvaluator(pe permission.PermissionEvaluator) Option {
	return func(srv *eventsServer) {
		srv.pe = pe
	}
}

type eventsServer struct {
	x.UnimplementedEventServiceHandler
	service *eventservice.EventService
	pe      permission.PermissionEvaluator
	pool    *pgxpool.Pool
}

//nolint:whitespace // can't make both editor and linter happy
func (s *eventsServer) GetEvents(
	ctx context.Context,
	req *connect.Request[eventv1.GetEventsRequest],
	stream *connect.ServerStream[eventv1.GetEventsResponse],
) error {
	data, err := event.LoadAll(context.Background(), s.pool)
	if err != nil {
		return err
	}
	for i := range data {

		if err := stream.Send(
			&eventv1.GetEventsResponse{Event: data[i]}); err != nil {
			log.Error("Error sending event", log.ErrorField(err))
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *eventsServer) GetLatestEvents(
	ctx context.Context,
	req *connect.Request[eventv1.GetLatestEventsRequest],
) (*connect.Response[eventv1.GetLatestEventsResponse], error) {
	data, err := event.LoadAll(context.Background(), s.pool)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&eventv1.GetLatestEventsResponse{Events: data}), nil
}

//nolint:whitespace,funlen // can't make both editor and linter happy
func (s *eventsServer) GetEvent(
	ctx context.Context, req *connect.Request[eventv1.GetEventRequest],
) (*connect.Response[eventv1.GetEventResponse], error) {
	log.Debug("GetEvent called",
		log.Any("arg", req.Msg),
		log.Int32("id", req.Msg.EventSelector.GetId()))
	var e *eventv1.Event
	var a *analysisv1.Analysis

	var err error
	e, err = util.ResolveEvent(ctx, s.pool, req.Msg.EventSelector)
	if err != nil {
		log.Error("error resolving event",
			log.Any("selector", req.Msg.EventSelector),
			log.ErrorField(err))
		return nil, err
	}

	a, err = aProto.LoadByEventId(ctx, s.pool, int(e.Id))
	if err != nil {
		log.Error("error loading event",
			log.Uint32("eventId", e.Id),
			log.ErrorField(err))
		return nil, err
	}

	t, err := track.LoadById(ctx, s.pool, int(e.TrackId))
	if err != nil {
		log.Error("error loading track",
			log.Uint32("eventId", e.Id),
			log.Uint32("trackId", e.TrackId),
			log.ErrorField(err))
		return nil, err
	}
	cd, err := cProto.LoadLatest(ctx, s.pool, int(e.Id))
	if err != nil {
		log.Error("error loading car proto data",
			log.Uint32("eventId", e.Id),
			log.ErrorField(err))
		return nil, err
	}
	sd, err := rProto.LoadLatest(ctx, s.pool, int(e.Id))
	if err != nil {
		log.Error("error loading race proto data",
			log.Uint32("eventId", e.Id),
			log.ErrorField(err))
		return nil, err
	}
	m, err := rProto.CollectMessages(ctx, s.pool, int(e.Id))
	if err != nil {
		log.Error("error collecting messages",
			log.Uint32("eventId", e.Id),
			log.ErrorField(err))
		return nil, err
	}
	log.Debug("message collected", log.Int("num", len(m)))
	sm, err := smProto.LoadLatest(ctx, s.pool, int(e.Id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			sm = &racestatev1.PublishSpeedmapRequest{}
		} else {
			log.Error("error loading speedmap proto data",
				log.Uint32("eventId", e.Id),
				log.ErrorField(err))
			return nil, err
		}
	}

	snapshots, err := smProto.LoadSnapshots(ctx, s.pool, int(e.Id), 120)
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
	if !s.pe.HasRole(a, auth.RoleProvider) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}
	log.Debug("DeleteEvent called",
		log.Any("arg", req.Msg),
		log.Int32("id", req.Msg.EventSelector.GetId()))
	var data *eventv1.Event
	var err error
	data, err = util.ResolveEvent(ctx, s.pool, req.Msg.EventSelector)
	if err != nil {
		return nil, err
	}

	err = s.service.DeleteEvent(ctx, int(data.Id))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&eventv1.DeleteEventResponse{}), nil
}
