package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/provider/v1/providerv1connect"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	containerv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/container/v1"
	providerv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/provider/v1"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	serverUtil "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/util"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
	"github.com/mpapenbr/iracelog-service-manager-go/version"
)

func NewServer(opts ...Option) *providerServer {
	ret := &providerServer{log: log.Default().Named("grpc.provider")}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type Option func(*providerServer)

func WithRepositories(arg api.Repositories) Option {
	return func(srv *providerServer) {
		srv.repos = arg
	}
}

func WithTxManager(txMgr api.TransactionManager) Option {
	return func(srv *providerServer) {
		srv.txMgr = txMgr
	}
}

func WithEventLookup(lookup *utils.EventLookup) Option {
	return func(srv *providerServer) {
		srv.lookup = lookup
	}
}

func WithDataProxy(arg proxy.DataProxy) Option {
	return func(srv *providerServer) {
		srv.dataProxy = arg
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
	dataProxy proxy.DataProxy
	repos     api.Repositories
	txMgr     api.TransactionManager
	pe        permission.PermissionEvaluator
	lookup    *utils.EventLookup
	log       *log.Logger
}

//nolint:whitespace // can't make both editor and linter happy
func (s *providerServer) ListLiveEvents(
	ctx context.Context,
	req *connect.Request[providerv1.ListLiveEventsRequest],
) (*connect.Response[providerv1.ListLiveEventsResponse], error) {
	t, err := serverUtil.ResolveTenant(ctx, s.repos.Tenant(), req.Msg.TenantSelector)
	if err != nil {
		return nil, err
	}
	s.log.Debug("ListLiveEvents called")
	ec := []*providerv1.LiveEventContainer{}
	for _, v := range s.dataProxy.LiveEvents() {
		if t == nil {
			ec = append(ec, &providerv1.LiveEventContainer{Event: v.Event, Track: v.Track})
		} else if t.Tenant.Name == v.Owner {
			ec = append(ec, &providerv1.LiveEventContainer{Event: v.Event, Track: v.Track})
		}
	}
	return connect.NewResponse(&providerv1.ListLiveEventsResponse{Events: ec}), nil
}

//nolint:whitespace,funlen // can't make both editor and linter happy
func (s *providerServer) RegisterEvent(
	ctx context.Context,
	req *connect.Request[providerv1.RegisterEventRequest],
) (*connect.Response[providerv1.RegisterEventResponse], error) {
	s.log.Debug("RegisterEvent called", log.Any("header", req.Header()))
	a := auth.FromContext(&ctx)
	if !s.pe.HasPermission(a, permission.PermissionRegisterEvent) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}

	s.log.Debug("RegisterEvent",
		log.Any("track", req.Msg.Track),
		log.Any("event", req.Msg.Event),
		log.String("tenant", a.Principal().Name()),
	)

	selector := &commonv1.EventSelector{
		Arg: &commonv1.EventSelector_Key{
			Key: req.Msg.Key,
		},
	}
	if ed, _ := s.dataProxy.GetEvent(selector); ed != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, ErrEventAlreadyRegistered)
	}

	if err := s.storeData(
		ctx,
		req.Msg.RecordingMode,
		func(ctx context.Context) error {
			if err := s.repos.Track().EnsureTrack(ctx, req.Msg.Track); err != nil {
				return err
			}
			if ta, ok := a.(auth.TenantAuthentication); ok {
				s.log.Debug("tenant id", log.Uint32("id", ta.GetID()))
				return s.repos.Event().Create(ctx, req.Msg.Event, ta.GetID())
			}
			return fmt.Errorf("no tenant id found in auth")
		}); err != nil {
		s.log.Error("error creating data", log.ErrorField(err))
		return nil, err
	}
	// read track from db to include pit stop info if already there
	dbTrack, err := s.repos.Track().LoadByID(ctx, int(req.Msg.Event.TrackId))
	if err != nil {
		s.log.Error("error loading track", log.ErrorField(err))
		dbTrack = req.Msg.Track
	}
	epd := s.lookup.AddEvent(
		req.Msg.Event,
		dbTrack,
		req.Msg.RecordingMode,
		a.Principal().Name())
	s.storeAnalysisDataWorker(epd)
	s.storeReplayInfoWorker(epd)
	if err = s.dataProxy.PublishEventRegistered(epd); err != nil {
		s.log.Error("error publishing registered event", log.ErrorField(err))
	}
	s.log.Debug("event registered")
	return connect.NewResponse(&providerv1.RegisterEventResponse{
			Event: req.Msg.Event,
			Track: dbTrack,
		}),
		nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *providerServer) UnregisterEvent(
	ctx context.Context,
	req *connect.Request[providerv1.UnregisterEventRequest],
) (*connect.Response[providerv1.UnregisterEventResponse], error) {
	s.log.Debug("UnregisterEvent",
		log.Any("event", req.Msg.EventSelector))
	a := auth.FromContext(&ctx)
	ed, err := s.validateEventAccess(a, req.Msg.EventSelector)
	if err != nil {
		return nil, err
	}
	epd, _ := s.lookup.GetEvent(req.Msg.EventSelector)
	if epd != nil {
		s.log.Debug("I was processing this event", log.String("key", epd.Event.Key))

		s.storeAnalysisData(epd)
		s.storeReplayInfo(epd)
		s.lookup.RemoveEvent(req.Msg.EventSelector)
	}
	if err := s.dataProxy.PublishEventUnregistered(ed.Event.Key); err != nil {
		s.log.Error("error publishing unregistered event", log.ErrorField(err))
	}
	s.log.Debug("Event unregistered",
		log.Any("event", req.Msg.EventSelector))
	return connect.NewResponse(&providerv1.UnregisterEventResponse{}), nil
}

func (s *providerServer) validateEventAccess(
	a auth.Authentication, eventSel *commonv1.EventSelector,
) (*containerv1.EventContainer, error) {
	// get the ed
	ed, err := s.dataProxy.GetEvent(eventSel)
	if err != nil {
		if !s.pe.HasPermission(a, permission.PermissionPostRacedata) {
			return nil, connect.NewError(
				connect.CodePermissionDenied,
				auth.ErrPermissionDenied)
		} else {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
	}
	if !s.pe.HasObjectPermission(a,
		permission.PermissionPostRacedata,
		ed.Owner) {

		return nil, connect.NewError(
			connect.CodePermissionDenied,
			auth.ErrPermissionDenied)
	}
	return ed, nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *providerServer) UnregisterAll(
	ctx context.Context,
	req *connect.Request[providerv1.UnregisterAllRequest],
) (*connect.Response[providerv1.UnregisterAllResponse], error) {
	a := auth.FromContext(&ctx)
	if !s.pe.HasPermission(a, permission.PermissionAdminUnregisterAllEvents) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}
	// TODO: data has to retrieved by pubsubData
	ec := []*providerv1.LiveEventContainer{}
	for _, v := range s.lookup.GetEvents() {
		s.storeAnalysisData(v)
		ec = append(ec, &providerv1.LiveEventContainer{Event: v.Event, Track: v.Track})
	}
	s.lookup.Clear()
	return connect.NewResponse(&providerv1.UnregisterAllResponse{Events: ec}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *providerServer) Ping(
	ctx context.Context,
	req *connect.Request[providerv1.PingRequest],
) (*connect.Response[providerv1.PingResponse], error) {
	return connect.NewResponse(&providerv1.PingResponse{
		Num:       req.Msg.Num,
		Timestamp: timestamppb.Now(),
	}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *providerServer) VersionCheck(
	ctx context.Context,
	req *connect.Request[providerv1.VersionCheckRequest],
) (*connect.Response[providerv1.VersionCheckResponse], error) {
	a := auth.FromContext(&ctx)

	return connect.NewResponse(&providerv1.VersionCheckResponse{
		ProvidedRaceloggerVersion:  req.Msg.RaceloggerVersion,
		SupportedRaceloggerVersion: util.RequiredClientVersion,
		ServerVersion:              version.Version,
		RaceloggerCompatible:       util.CheckRaceloggerVersion(req.Msg.RaceloggerVersion),
		ValidCredentials:           s.pe.HasPermission(a, permission.PermissionPostRacedata),
	}), nil
}

//nolint:whitespace,dupl // false positive
func (s *providerServer) storeAnalysisDataWorker(
	epd *utils.EventProcessingData,
) {
	if epd.RecordingMode == providerv1.RecordingMode_RECORDING_MODE_DO_NOT_PERSIST {
		return
	}
	// in case we're recording the data, setup a listener for the analysis data
	// and persist it to the database every 15s
	go func() {
		ch := epd.AnalysisBroadcast.Subscribe()
		lastPersist := time.Now()
		for data := range ch {
			//nolint:errcheck // by design
			if time.Since(lastPersist) > 15*time.Second {
				lastPersist = time.Now()
				if err := s.txMgr.RunInTx(context.Background(),
					func(ctx context.Context) error {
						return s.repos.Analysis().Upsert(
							ctx,
							int(epd.Event.Id),
							data)
					}); err != nil {
					s.log.Error("error storing analysis data", log.ErrorField(err))
				}
			}
		}
	}()
}

//nolint:whitespace,dupl // false positive
func (s *providerServer) storeReplayInfoWorker(
	epd *utils.EventProcessingData,
) {
	if epd.RecordingMode == providerv1.RecordingMode_RECORDING_MODE_DO_NOT_PERSIST {
		return
	}
	// in case we're recording the data, setup a listener for the analysis data
	// and persist it to the database every 5s
	go func() {
		ch := epd.ReplayInfoBroadcast.Subscribe()
		lastPersist := time.Now()
		for data := range ch {
			//nolint:errcheck // by design
			if time.Since(lastPersist) > 5*time.Second {
				lastPersist = time.Now()
				if err := s.txMgr.RunInTx(context.Background(),
					func(ctx context.Context) error {
						return s.repos.Event().UpdateReplayInfo(
							ctx,
							int(epd.Event.Id),
							data)
					}); err != nil {
					s.log.Error("error storing replay info data", log.ErrorField(err))
				}
			}
		}
	}()
}

//nolint:whitespace,dupl // false positive
func (s *providerServer) storeAnalysisData(
	epd *utils.EventProcessingData,
) {
	if err := s.storeData(
		context.Background(),
		epd.RecordingMode,
		func(ctx context.Context) error {
			return s.repos.Analysis().Upsert(
				ctx,
				int(epd.Event.Id),
				epd.LastAnalysisData)
		}); err != nil {
		s.log.Error("error storing analysis data", log.ErrorField(err))
	}
}

//nolint:whitespace,dupl // false positive
func (s *providerServer) storeReplayInfo(
	epd *utils.EventProcessingData,
) {
	if err := s.storeData(
		context.Background(),
		epd.RecordingMode,
		func(ctx context.Context) error {
			return s.repos.Event().UpdateReplayInfo(
				ctx,
				int(epd.Event.Id),
				epd.LastReplayInfo)
		}); err != nil {
		s.log.Error("error storing replay info", log.ErrorField(err))
	}
}

// helper function to store data in the database within a transaction
// function evalates epd.RecordingMode to determine if data should be stored
//
//nolint:whitespace // can't make both editor and linter happy
func (s *providerServer) storeData(
	ctx context.Context,
	recordingMode providerv1.RecordingMode,
	storeFunc func(ctx context.Context) error,
) error {
	if recordingMode == providerv1.RecordingMode_RECORDING_MODE_DO_NOT_PERSIST {
		return nil
	}
	return s.txMgr.RunInTx(ctx, func(ctx context.Context) error {
		if err := storeFunc(ctx); err != nil {
			return err
		}
		return nil
	})
}
