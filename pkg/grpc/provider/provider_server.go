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
)

func NewServer(opts ...Option) *providerServer {
	ret := &providerServer{
		lookup: make(map[string]*eventv1.Event),
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type Option func(*providerServer)

func WithPool(p *pgxpool.Pool) Option {
	return func(srv *providerServer) {
		srv.pool = p
	}
}

func WithPermissionEvaluator(pe permission.PermissionEvaluator) Option {
	return func(srv *providerServer) {
		srv.pe = pe
	}
}

var (
	ErrEventAlreadyRegistered = errors.New("event already registered")
	ErrEventNotFound          = errors.New("event not found")
)

type providerServer struct {
	x.UnimplementedProviderServiceHandler
	pool   *pgxpool.Pool
	pe     permission.PermissionEvaluator
	lookup map[string]*eventv1.Event
}

//nolint:whitespace // can't make both editor and linter happy
func (s *providerServer) ListLiveEvents(
	ctx context.Context,
	req *connect.Request[providerv1.ListLiveEventsRequest],
	stream *connect.ServerStream[providerv1.ListLiveEventsResponse],
) error {
	log.Debug("ListLiveEvents called")
	for _, v := range s.lookup {
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

	if _, ok := s.lookup[req.Msg.Event.Key]; ok {
		return nil, connect.NewError(connect.CodeAlreadyExists, ErrEventAlreadyRegistered)
	}
	s.lookup[req.Msg.Event.Key] = req.Msg.Event
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
	var key string
	switch req.Msg.EventSelector.Arg.(type) {
	case *eventv1.EventSelector_Id:
		for k, v := range s.lookup {
			if v.Id == uint32(req.Msg.EventSelector.GetId()) {
				key = k
				break
			}
		}
	case *eventv1.EventSelector_Key:
		key = req.Msg.EventSelector.GetKey()
	}
	if key == "" {
		return nil, connect.NewError(connect.CodeNotFound, ErrEventNotFound)
	}
	s.cleanup(key)
	delete(s.lookup, key)
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
	ret := make([]*eventv1.Event, len(s.lookup))
	i := 0
	for k, v := range s.lookup {
		ret[i] = v
		s.cleanup(k)
		i++
		if err := stream.Send(&providerv1.UnregisterAllResponse{Event: v}); err != nil {
			log.Warn("Error sending event on unregisterAll", log.ErrorField(err))
		}
	}
	s.lookup = make(map[string]*eventv1.Event)
	return nil
}

func (s *providerServer) cleanup(key string) {
	log.Warn("cleanup not yet implemented", log.String("key", key))
}
