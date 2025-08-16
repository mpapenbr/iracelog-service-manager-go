package state

import (
	"context"
	"slices"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/racestate/v1/racestatev1connect"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	providerv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/provider/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"connectrpc.com/connect"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy"
	mainUtil "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/util"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewServer(opts ...Option) *stateServer {
	ret := &stateServer{
		debugWire: false,
		log:       log.Default().Named("grpc.state"),
		wireLog:   log.Default().Named("grpc.state.wire"),
	}
	for _, opt := range opts {
		opt(ret)
	}
	if ret.tracer == nil {
		ret.tracer = otel.Tracer("ism")
	}
	return ret
}

type Option func(*stateServer)

func WithRepositories(r api.Repositories) Option {
	return func(srv *stateServer) {
		srv.repos = r
	}
}

func WithTxManager(txMgr api.TransactionManager) Option {
	return func(srv *stateServer) {
		srv.txMgr = txMgr
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

func WithDebugWire(arg bool) Option {
	return func(srv *stateServer) {
		srv.debugWire = arg
	}
}

func WithDataProxy(pd proxy.DataProxy) Option {
	return func(srv *stateServer) {
		srv.dataProxy = pd
	}
}

func WithTracer(tracer trace.Tracer) Option {
	return func(srv *stateServer) {
		srv.tracer = tracer
	}
}

type stateServer struct {
	x.UnimplementedRaceStateServiceHandler
	repos     api.Repositories
	txMgr     api.TransactionManager
	pe        permission.PermissionEvaluator
	dataProxy proxy.DataProxy
	lookup    *utils.EventLookup
	log       *log.Logger
	wireLog   *log.Logger
	debugWire bool // if true, debug events affecting "wire" actions (send/receive)
	tracer    trace.Tracer
}

//nolint:whitespace // can't make both editor and linter happy
func (s *stateServer) PublishState(
	ctx context.Context,
	req *connect.Request[racestatev1.PublishStateRequest]) (
	*connect.Response[racestatev1.PublishStateResponse], error,
) {
	a := auth.FromContext(&ctx)
	// get the epd
	epd, err := s.validateEventAccess(a, req.Msg.Event)
	if err != nil {
		return nil, err
	}
	if s.debugWire {
		s.wireLog.Debug("PublishState called",
			log.String("event", epd.Event.Key),
			log.Int("car entries", len(req.Msg.Cars)))
	}
	if err := s.dataProxy.PublishRaceStateData(req.Msg); err != nil {
		s.log.Error("error publishing state", log.ErrorField(err))
	}
	if s.isRaceSession(epd, req.Msg) {
		if err := s.storeData(ctx, epd,
			func(ctx context.Context) error {
				id, err := s.repos.Racestate().CreateRacestate(
					ctx, int(epd.Event.Id), req.Msg)
				if err == nil {
					epd.LastRsInfoID = id
				}
				return err
			}); err != nil {
			s.log.Error("error storing state", log.ErrorField(err))
		}
	}
	epd.Mutex.Lock()
	epd.MarkDataEvent()
	defer epd.Mutex.Unlock()
	epd.Processor.ProcessState(req.Msg)
	return connect.NewResponse(&racestatev1.PublishStateResponse{}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *stateServer) isRaceSession(
	edp *utils.EventProcessingData,
	req *racestatev1.PublishStateRequest,
) bool {
	ret := slices.Contains(edp.RaceSessions, req.Session.SessionNum)
	if !ret {
		if s.debugWire {
			s.log.Debug("not in race session",
				log.Uint32("session", req.Session.SessionNum),
				log.Any("raceSessions", edp.RaceSessions))
		}
	}
	return ret
}

//nolint:whitespace // can't make both editor and linter happy
func (s *stateServer) PublishSpeedmap(
	ctx context.Context,
	req *connect.Request[racestatev1.PublishSpeedmapRequest]) (
	*connect.Response[racestatev1.PublishSpeedmapResponse], error,
) {
	a := auth.FromContext(&ctx)
	// get the epd
	epd, err := s.validateEventAccess(a, req.Msg.Event)
	if err != nil {
		return nil, err
	}
	if s.debugWire {
		s.wireLog.Debug("PublishSpeedmap called",
			log.String("event", epd.Event.Key),
			log.Int("speedmap map entries", len(req.Msg.Speedmap.Data)))
	}

	if err := s.dataProxy.PublishSpeedmapData(req.Msg); err != nil {
		s.log.Error("error publishing speedmap", log.ErrorField(err))
	}
	if err := s.storeData(ctx, epd, func(ctx context.Context) error {
		return s.repos.Speedmap().Create(ctx, epd.LastRsInfoID, req.Msg)
	}); err != nil {
		s.log.Error("error storing speedmap", log.ErrorField(err))
	}

	epd.Processor.ProcessSpeedmap(req.Msg)

	return connect.NewResponse(&racestatev1.PublishSpeedmapResponse{}), nil
}

//nolint:whitespace,funlen // can't make both editor and linter happy
func (s *stateServer) PublishDriverData(
	ctx context.Context,
	req *connect.Request[racestatev1.PublishDriverDataRequest]) (
	*connect.Response[racestatev1.PublishDriverDataResponse], error,
) {
	a := auth.FromContext(&ctx)
	// get the epd
	epd, err := s.validateEventAccess(a, req.Msg.Event)
	if err != nil {
		return nil, err
	}

	if s.debugWire {
		s.wireLog.Debug("PublishDriverData called", log.String("event", epd.Event.Key))
	}

	if err := s.dataProxy.PublishDriverData(req.Msg); err != nil {
		s.log.Error("error publishing state", log.ErrorField(err))
	}
	if err := s.storeData(ctx, epd, func(ctx context.Context) error {
		// Note: carrepos uses eventId, carprotorepos rsInfoId
		if err := s.repos.Car().Create(ctx, int(epd.Event.Id), req.Msg); err != nil {
			s.log.Error("error storing car data", log.ErrorField(err))
		}
		rsInfoID := epd.LastRsInfoID
		if epd.LastRsInfoID == 0 {
			var rsErr error
			rsInfoID, rsErr = s.repos.Racestate().CreateDummyRacestateInfo(
				ctx, int(epd.Event.Id), req.Msg.Timestamp.AsTime(),
				req.Msg.SessionTime, req.Msg.SessionNum,
			)
			if rsErr != nil {
				s.log.Error("error creating dummy racestate info", log.ErrorField(rsErr))
				return rsErr
			}
		}
		return s.repos.CarProto().Create(ctx, rsInfoID, req.Msg)
	}); err != nil {
		s.log.Error("error storing car state", log.ErrorField(err))
	}

	epd.Mutex.Lock()
	defer epd.Mutex.Unlock()
	epd.Processor.ProcessCarData(req.Msg)

	return connect.NewResponse(&racestatev1.PublishDriverDataResponse{}), nil
}

func (s *stateServer) validateEventAccess(
	a auth.Authentication, eventSel *commonv1.EventSelector,
) (*utils.EventProcessingData, error) {
	// get the epd
	epd, err := s.lookup.GetEvent(eventSel)
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
		epd.Owner) {

		return nil, connect.NewError(
			connect.CodePermissionDenied,
			auth.ErrPermissionDenied)
	}
	return epd, nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *stateServer) PublishEventExtraInfo(
	ctx context.Context,
	req *connect.Request[racestatev1.PublishEventExtraInfoRequest]) (
	*connect.Response[racestatev1.PublishEventExtraInfoResponse], error,
) {
	a := auth.FromContext(&ctx)
	// get the epd
	epd, err := s.validateEventAccess(a, req.Msg.Event)
	if err != nil {
		return nil, err
	}
	if s.debugWire {
		s.wireLog.Debug("PublishEventExtraInfo called", log.String("event", epd.Event.Key))
	}

	if err := s.storeData(ctx, epd, func(ctx context.Context) error {
		s.handlePitInfoUpdate(ctx, int(epd.Event.TrackId), req.Msg.ExtraInfo.PitInfo)

		return s.repos.EventExt().Upsert(ctx, int(epd.Event.Id), req.Msg.ExtraInfo)
	}); err != nil {
		s.log.Error("error storing event extra info", log.ErrorField(err))
	}

	return connect.NewResponse(&racestatev1.PublishEventExtraInfoResponse{}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *stateServer) handlePitInfoUpdate(
	ctx context.Context,
	trackID int,
	pitInfo *trackv1.PitInfo,
) {
	if pitInfo == nil {
		return
	}
	if pitInfo.Entry == 0 && pitInfo.Exit == 0 && pitInfo.LaneLength == 0 {
		return
	}
	//nolint:nestif // by design
	if t, err := s.repos.Track().LoadByID(ctx, trackID); err != nil {
		s.log.Error("error loading track", log.ErrorField(err))
		return
	} else {
		if t.PitInfo == nil {
			t.PitInfo = &trackv1.PitInfo{}
		} else if t.PitInfo.LaneLength != 0 {
			// pit info already set, do not overwrite
			return
		}
		t.PitInfo.Entry = pitInfo.Entry
		t.PitInfo.Exit = pitInfo.Exit
		t.PitInfo.LaneLength = pitInfo.LaneLength
		_, err := s.repos.Track().UpdatePitInfo(ctx, trackID, t.PitInfo)
		if err != nil {
			s.log.Error("error updating pit info", log.ErrorField(err))
		}
	}
}

//nolint:whitespace,dupl // false positive
func (s *stateServer) GetDriverData(
	ctx context.Context,
	req *connect.Request[racestatev1.GetDriverDataRequest]) (
	*connect.Response[racestatev1.GetDriverDataResponse], error,
) {
	var sc *driverDataContainer
	var err error
	if sc, err = CreateDriverDataContainer(ctx, s.repos, req.Msg); err != nil {
		return nil, err
	}
	var tmp *mainUtil.RangeContainer[racestatev1.PublishDriverDataRequest]
	tmp, err = sc.InitialRequest()
	if err != nil {
		return nil, err
	}
	// Note: we don't want to return more data then configured in driverDataContainer
	// if a client needs more, it should call again or use the stream method
	return connect.NewResponse(&racestatev1.GetDriverDataResponse{
		DriverData:      tmp.Data,
		LastTs:          timestamppb.New(tmp.LastTimestamp),
		LastSessionTime: tmp.LastSessionTime,
		LastId:          uint32(tmp.LastRsInfoID),
	}), nil
}

//nolint:whitespace,dupl // false positive
func (s *stateServer) GetDriverDataStream(
	ctx context.Context,
	req *connect.Request[racestatev1.GetDriverDataStreamRequest],
	stream *connect.ServerStream[racestatev1.GetDriverDataStreamResponse],
) (err error) {
	var sc *driverDataContainer
	if sc, err = CreateDriverDataContainer(ctx, s.repos, req.Msg); err != nil {
		return err
	}
	var rc *mainUtil.RangeContainer[racestatev1.PublishDriverDataRequest]
	rc, err = sc.InitialRequest()
	for {
		if err != nil {
			return err
		}
		if len(rc.Data) == 0 {
			s.log.Debug("GetDriverDataStream no more data",
				log.String("event", sc.e.GetKey()))
			return nil
		}
		for _, s := range rc.Data {
			if sErr := stream.Send(&racestatev1.GetDriverDataStreamResponse{
				DriverData: s,
			}); sErr != nil {
				return sErr
			}
		}
		rc, err = sc.NextRequest()
	}
}

//nolint:whitespace,dupl // false positive
func (s *stateServer) GetStates(
	ctx context.Context,
	req *connect.Request[racestatev1.GetStatesRequest]) (
	*connect.Response[racestatev1.GetStatesResponse], error,
) {
	var sc *racestatesContainer
	var err error
	if sc, err = CreateRacestatesContainer(ctx, s.repos, req.Msg); err != nil {
		return nil, err
	}
	var tmp *mainUtil.RangeContainer[racestatev1.PublishStateRequest]
	tmp, err = sc.InitialRequest()
	if err != nil {
		return nil, err
	}
	// Note: we don't want to return more data then configured in statesContainer
	// if a client needs more, it should call again or use the stream method
	return connect.NewResponse(&racestatev1.GetStatesResponse{
		States:          tmp.Data,
		LastTs:          timestamppb.New(tmp.LastTimestamp),
		LastSessionTime: tmp.LastSessionTime,
		LastId:          uint32(tmp.LastRsInfoID),
	}), nil
}

//nolint:whitespace,dupl // false positive
func (s *stateServer) GetStateStream(
	ctx context.Context,
	req *connect.Request[racestatev1.GetStateStreamRequest],
	stream *connect.ServerStream[racestatev1.GetStateStreamResponse],
) (err error) {
	var sc *racestatesContainer
	if sc, err = CreateRacestatesContainer(ctx, s.repos, req.Msg); err != nil {
		return err
	}
	var rc *mainUtil.RangeContainer[racestatev1.PublishStateRequest]
	rc, err = sc.InitialRequest()
	for {
		if err != nil {
			return err
		}
		if len(rc.Data) == 0 {
			s.log.Debug("GetStateStream no more data", log.String("event", sc.e.GetKey()))
			return nil
		}
		for _, s := range rc.Data {
			if sErr := stream.Send(&racestatev1.GetStateStreamResponse{
				State: s,
			}); sErr != nil {
				return sErr
			}
		}
		rc, err = sc.NextRequest()
	}
}

//nolint:whitespace,dupl // false positive
func (s *stateServer) GetSpeedmaps(
	ctx context.Context,
	req *connect.Request[racestatev1.GetSpeedmapsRequest]) (
	*connect.Response[racestatev1.GetSpeedmapsResponse], error,
) {
	var sc *speedmapContainer
	var err error
	if sc, err = createSpeedmapContainer(ctx, s.repos, req.Msg); err != nil {
		return nil, err
	}
	var tmp *mainUtil.RangeContainer[racestatev1.PublishSpeedmapRequest]
	tmp, err = sc.InitialRequest()
	if err != nil {
		return nil, err
	}
	// Note: we don't want to return more data then configured in speedmapContainer
	// if a client needs more, it should call again or use the stream method
	return connect.NewResponse(&racestatev1.GetSpeedmapsResponse{
		Speedmaps:       tmp.Data,
		LastTs:          timestamppb.New(tmp.LastTimestamp),
		LastSessionTime: tmp.LastSessionTime,
		LastId:          uint32(tmp.LastRsInfoID),
	}), nil
}

//nolint:whitespace,dupl // false positive
func (s *stateServer) GetSpeedmapStream(
	ctx context.Context,
	req *connect.Request[racestatev1.GetSpeedmapStreamRequest],
	stream *connect.ServerStream[racestatev1.GetSpeedmapStreamResponse],
) (err error) {
	var sc *speedmapContainer
	if sc, err = createSpeedmapContainer(ctx, s.repos, req.Msg); err != nil {
		return err
	}
	var rc *mainUtil.RangeContainer[racestatev1.PublishSpeedmapRequest]
	rc, err = sc.InitialRequest()
	for {
		if err != nil {
			return err
		}
		if len(rc.Data) == 0 {
			s.log.Debug("GetSpeedmapStream no more data", log.String("event", sc.e.GetKey()))
			return nil
		}
		for _, s := range rc.Data {
			if sErr := stream.Send(&racestatev1.GetSpeedmapStreamResponse{
				Speedmap: s,
			}); sErr != nil {
				return sErr
			}
		}
		rc, err = sc.NextRequest()
	}
}

// helper function to store data in the database within a transaction
// function evalates epd.RecordingMode to determine if data should be stored
//
//nolint:whitespace // can't make both editor and linter happy
func (s *stateServer) storeData(
	ctx context.Context,
	epd *utils.EventProcessingData,
	storeFunc func(ctx context.Context) error,
) error {
	if epd.RecordingMode == providerv1.RecordingMode_RECORDING_MODE_DO_NOT_PERSIST {
		return nil
	}
	return s.txMgr.RunInTx(ctx, func(ctx context.Context) error {
		if err := storeFunc(ctx); err != nil {
			return err
		}
		return nil
	})
}
