package state

import (
	"context"

	x "buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/racestate/v1/racestatev1connect"
	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewServer(opts ...Option) *stateServer {
	ret := &stateServer{}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type Option func(*stateServer)

func WithPool(p *pgxpool.Pool) Option {
	return func(srv *stateServer) {
		srv.pool = p
	}
}

func WithPermissionEvaluator(pe permission.PermissionEvaluator) Option {
	return func(srv *stateServer) {
		srv.pe = pe
	}
}

func WithEventLookup(lookup *utils.EventLookup) Option {
	return func(srv *stateServer) {
		srv.lookup = lookup
	}
}

type stateServer struct {
	x.UnimplementedRaceStateServiceHandler
	pool   *pgxpool.Pool
	pe     permission.PermissionEvaluator
	lookup *utils.EventLookup
}

//nolint:whitespace // can't make both editor and linter happy
func (s *stateServer) PublishState(
	ctx context.Context,
	req *connect.Request[racestatev1.PublishStateRequest]) (
	*connect.Response[racestatev1.PublishStateResponse], error,
) {
	a := auth.FromContext(&ctx)
	if !s.pe.HasRole(a, auth.RoleProvider) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}
	// get the epd
	epd, err := s.lookup.GetEvent(req.Msg.Event)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	log.Debug("PublishState called",
		log.String("event", epd.Event.Key),
		log.Int("car entries", len(req.Msg.Cars)))
	epd.Processor.ProcessState(req.Msg)
	return connect.NewResponse(&racestatev1.PublishStateResponse{}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *stateServer) PublishSpeedmap(
	ctx context.Context,
	req *connect.Request[racestatev1.PublishSpeedmapRequest]) (
	*connect.Response[racestatev1.PublishSpeedmapResponse], error,
) {
	a := auth.FromContext(&ctx)
	if !s.pe.HasRole(a, auth.RoleProvider) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}
	// get the epd
	epd, err := s.lookup.GetEvent(req.Msg.Event)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	log.Debug("PublishSpeedmap called",
		log.String("event", epd.Event.Key),
		log.Int("speedmap map entries", len(req.Msg.Speedmap.Data)))
	return connect.NewResponse(&racestatev1.PublishSpeedmapResponse{}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *stateServer) PublishDriverData(
	ctx context.Context,
	req *connect.Request[racestatev1.PublishDriverDataRequest]) (
	*connect.Response[racestatev1.PublishDriverDataResponse], error,
) {
	a := auth.FromContext(&ctx)
	if !s.pe.HasRole(a, auth.RoleProvider) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}
	// get the epd
	epd, err := s.lookup.GetEvent(req.Msg.Event)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	log.Debug("PublishDriverData called",
		log.String("event", epd.Event.Key))
	epd.Processor.ProcessCarData(req.Msg)
	return connect.NewResponse(&racestatev1.PublishDriverDataResponse{}), nil
}
