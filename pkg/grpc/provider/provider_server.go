package provider

import (
	"context"
	"errors"

	x "buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/provider/v1/providerv1connect"
	eventv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/event/v1"
	providerv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/provider/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewServer(opts ...Option) *providerServer {
	ret := &providerServer{}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type Option func(*providerServer)

func WithPersistence(p *pgxpool.Pool) Option {
	return func(srv *providerServer) {
		srv.pool = p
	}
}

func WithEventLookup(lookup *utils.EventLookup) Option {
	return func(srv *providerServer) {
		srv.lookup = lookup
	}
}

func WithPermissionEvaluator(pe permission.PermissionEvaluator) Option {
	return func(srv *providerServer) {
		srv.pe = pe
	}
}

var ErrEventAlreadyRegistered = errors.New("event already registered")

type providerServer struct {
	x.UnimplementedProviderServiceHandler
	pool   *pgxpool.Pool
	pe     permission.PermissionEvaluator
	lookup *utils.EventLookup
}

//nolint:whitespace // can't make both editor and linter happy
func (s *providerServer) ListLiveEvents(
	ctx context.Context,
	req *connect.Request[providerv1.ListLiveEventsRequest],
	stream *connect.ServerStream[providerv1.ListLiveEventsResponse],
) error {
	log.Debug("ListLiveEvents called")
	for _, v := range s.lookup.GetEvents() {
		if err := stream.Send(&providerv1.ListLiveEventsResponse{Event: v}); err != nil {
			log.Error("Error sending event", log.ErrorField(err))
			return err
		}
	}
	return nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *providerServer) RegisterEvent(
	ctx context.Context,
	req *connect.Request[providerv1.RegisterEventRequest],
) (*connect.Response[providerv1.RegisterEventResponse], error) {
	log.Debug("RegisterEvent called", log.Any("header", req.Header()))
	a := auth.FromContext(&ctx)
	if !s.pe.HasRole(a, auth.RoleProvider) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}

	log.Debug("RegisterEvent",
		log.Any("track", req.Msg.Track),
		log.Any("event", req.Msg.Event))

	selector := &eventv1.EventSelector{
		Arg: &eventv1.EventSelector_Key{
			Key: req.Msg.Key,
		},
	}
	if e, _ := s.lookup.GetEvent(selector); e != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, ErrEventAlreadyRegistered)
	}
	s.lookup.AddEvent(req.Msg.Event)
	return connect.NewResponse(&providerv1.RegisterEventResponse{Event: req.Msg.Event}),
		nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *providerServer) UnregisterEvent(
	ctx context.Context,
	req *connect.Request[providerv1.UnregisterEventRequest],
) (*connect.Response[providerv1.UnregisterEventResponse], error) {
	a := auth.FromContext(&ctx)
	if !s.pe.HasRole(a, auth.RoleProvider) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}

	event, err := s.lookup.GetEvent(req.Msg.EventSelector)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	s.cleanup(event)
	s.lookup.RemoveEvent(req.Msg.EventSelector)
	return connect.NewResponse(&providerv1.UnregisterEventResponse{}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *providerServer) UnregisterAll(
	ctx context.Context,
	req *connect.Request[providerv1.UnregisterAllRequest],
	stream *connect.ServerStream[providerv1.UnregisterAllResponse],
) error {
	a := auth.FromContext(&ctx)
	if !s.pe.HasRole(a, auth.RoleProvider) {
		return connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}

	for _, v := range s.lookup.GetEvents() {
		s.cleanup(v)
		if err := stream.Send(&providerv1.UnregisterAllResponse{Event: v}); err != nil {
			log.Warn("Error sending event on unregisterAll", log.ErrorField(err))
		}
	}
	s.lookup.Clear()
	return nil
}

func (s *providerServer) cleanup(event *eventv1.Event) {
	log.Warn("cleanup not yet implemented", log.String("key", event.Key))
}
