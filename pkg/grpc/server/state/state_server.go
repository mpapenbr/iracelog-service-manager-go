package state

import (
	"context"
	"slices"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/racestate/v1/racestatev1connect"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	providerv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/provider/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	carrepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/car"
	carprotorepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/car/proto"
	eventextrepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/event/ext"
	racestaterepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/racestate"
	speedmapprotorepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/speedmap/proto"
	trackrepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/track"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
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

func WithDebugWire(arg bool) Option {
	return func(srv *stateServer) {
		srv.debugWire = arg
	}
}

type stateServer struct {
	x.UnimplementedRaceStateServiceHandler
	pool      *pgxpool.Pool
	pe        permission.PermissionEvaluator
	lookup    *utils.EventLookup
	log       *log.Logger
	wireLog   *log.Logger
	debugWire bool // if true, debug events affecting "wire" actions (send/receive)
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
	if s.debugWire {
		s.wireLog.Debug("PublishState called",
			log.String("event", epd.Event.Key),
			log.Int("car entries", len(req.Msg.Cars)))
	}

	if s.isRaceSession(epd, req.Msg) {
		if err := s.storeData(ctx, epd, func(ctx context.Context, tx pgx.Tx) error {
			id, err := racestaterepos.CreateRaceState(ctx, tx, int(epd.Event.Id), req.Msg)
			if err == nil {
				epd.LastRsInfoId = id
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
	if !s.pe.HasRole(a, auth.RoleProvider) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}
	// get the epd
	epd, err := s.lookup.GetEvent(req.Msg.Event)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if s.debugWire {
		s.wireLog.Debug("PublishSpeedmap called",
			log.String("event", epd.Event.Key),
			log.Int("speedmap map entries", len(req.Msg.Speedmap.Data)))
	}

	if err := s.storeData(ctx, epd, func(ctx context.Context, tx pgx.Tx) error {
		return speedmapprotorepos.Create(ctx, tx, epd.LastRsInfoId, req.Msg)
	}); err != nil {
		s.log.Error("error storing speedmap", log.ErrorField(err))
	}

	epd.Processor.ProcessSpeedmap(req.Msg)

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
	if s.debugWire {
		s.wireLog.Debug("PublishDriverData called", log.String("event", epd.Event.Key))
	}

	if err := s.storeData(ctx, epd, func(ctx context.Context, tx pgx.Tx) error {
		// Note: carrepos uses eventId, carprotorepos rsInfoId
		if err := carrepos.Create(ctx, tx, int(epd.Event.Id), req.Msg); err != nil {
			s.log.Error("error storing car data", log.ErrorField(err))
		}
		if epd.LastRsInfoId == 0 {
			var rsErr error
			epd.LastRsInfoId, rsErr = racestaterepos.CreateDummyRaceStateInfo(
				ctx, tx, int(epd.Event.Id), req.Msg.Timestamp.AsTime())
			if rsErr != nil {
				s.log.Error("error creating dummy racestate info", log.ErrorField(rsErr))
				return rsErr
			}
		}
		return carprotorepos.Create(ctx, tx, epd.LastRsInfoId, req.Msg)
	}); err != nil {
		s.log.Error("error storing car state", log.ErrorField(err))
	}

	epd.Mutex.Lock()
	defer epd.Mutex.Unlock()
	epd.Processor.ProcessCarData(req.Msg)

	return connect.NewResponse(&racestatev1.PublishDriverDataResponse{}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *stateServer) PublishEventExtraInfo(
	ctx context.Context,
	req *connect.Request[racestatev1.PublishEventExtraInfoRequest]) (
	*connect.Response[racestatev1.PublishEventExtraInfoResponse], error,
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
	if s.debugWire {
		s.wireLog.Debug("PublishEventExtraInfo called", log.String("event", epd.Event.Key))
	}

	if err := s.storeData(ctx, epd, func(ctx context.Context, tx pgx.Tx) error {
		s.handlePitInfoUpdate(ctx, tx, int(epd.Event.TrackId), req.Msg.ExtraInfo.PitInfo)

		return eventextrepos.Upsert(ctx, tx, int(epd.Event.Id), req.Msg.ExtraInfo)
	}); err != nil {
		s.log.Error("error storing event extra info", log.ErrorField(err))
	}

	return connect.NewResponse(&racestatev1.PublishEventExtraInfoResponse{}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *stateServer) handlePitInfoUpdate(
	ctx context.Context,
	tx pgx.Tx,
	trackId int,
	pitInfo *trackv1.PitInfo,
) {
	if pitInfo == nil {
		return
	}
	if pitInfo.Entry == 0 && pitInfo.Exit == 0 && pitInfo.LaneLength == 0 {
		return
	}
	//nolint:nestif // by design
	if t, err := trackrepos.LoadById(ctx, tx, trackId); err != nil {
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
		_, err := trackrepos.UpdatePitInfo(ctx, tx, trackId, t.PitInfo)
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
	var data *eventv1.Event
	var err error
	data, err = util.ResolveEvent(ctx, s.pool, req.Msg.Event)
	if err != nil {
		return nil, err
	}
	var tmp *mainUtil.RangeContainer[racestatev1.PublishDriverDataRequest]
	switch req.Msg.Start.Arg.(type) {
	case *commonv1.StartSelector_RecordStamp:
		tmp, err = carprotorepos.LoadRange(
			ctx,
			s.pool,
			int(data.Id),
			req.Msg.Start.GetRecordStamp().AsTime(),
			int(req.Msg.Num))
	case *commonv1.StartSelector_SessionTime:
		tmp, err = carprotorepos.LoadRangeBySessionTime(
			ctx,
			s.pool,
			int(data.Id),
			float64(req.Msg.Start.GetSessionTime()),
			int(req.Msg.Num))

	}
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&racestatev1.GetDriverDataResponse{
		DriverData:      tmp.Data,
		LastTs:          timestamppb.New(tmp.LastTimestamp),
		LastSessionTime: tmp.LastSessionTime,
	}), nil
}

//nolint:whitespace,dupl // false positive
func (s *stateServer) GetStates(
	ctx context.Context,
	req *connect.Request[racestatev1.GetStatesRequest]) (
	*connect.Response[racestatev1.GetStatesResponse], error,
) {
	var data *eventv1.Event
	var err error
	data, err = util.ResolveEvent(ctx, s.pool, req.Msg.Event)
	if err != nil {
		return nil, err
	}
	var tmp *mainUtil.RangeContainer[racestatev1.PublishStateRequest]
	if req.Msg.Start == nil {
		return nil, util.ErrMissingEventSelector
	}
	switch req.Msg.Start.Arg.(type) {
	case *commonv1.StartSelector_RecordStamp:
		tmp, err = racestaterepos.LoadRange(
			ctx,
			s.pool,
			int(data.Id),
			req.Msg.Start.GetRecordStamp().AsTime(),
			int(req.Msg.Num))
	case *commonv1.StartSelector_SessionTime:
		tmp, err = racestaterepos.LoadRangeBySessionTime(
			ctx,
			s.pool,
			int(data.Id),
			float64(req.Msg.Start.GetSessionTime()),
			int(req.Msg.Num))
	default:
		err = util.ErrInvalidStartSelector

	}
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&racestatev1.GetStatesResponse{
		States:          tmp.Data,
		LastTs:          timestamppb.New(tmp.LastTimestamp),
		LastSessionTime: tmp.LastSessionTime,
	}), nil
}

//nolint:whitespace,dupl // false positive
func (s *stateServer) GetSpeedmaps(
	ctx context.Context,
	req *connect.Request[racestatev1.GetSpeedmapsRequest]) (
	*connect.Response[racestatev1.GetSpeedmapsResponse], error,
) {
	var data *eventv1.Event
	var err error
	data, err = util.ResolveEvent(ctx, s.pool, req.Msg.Event)
	if err != nil {
		return nil, err
	}

	var tmp *mainUtil.RangeContainer[racestatev1.PublishSpeedmapRequest]

	if req.Msg.Start == nil {
		return nil, util.ErrMissingStartSelector
	}
	switch req.Msg.Start.Arg.(type) {
	case *commonv1.StartSelector_RecordStamp:
		tmp, err = speedmapprotorepos.LoadRange(
			ctx,
			s.pool,
			int(data.Id),
			req.Msg.Start.GetRecordStamp().AsTime(),
			int(req.Msg.Num))
	case *commonv1.StartSelector_SessionTime:
		tmp, err = speedmapprotorepos.LoadRangeBySessionTime(
			ctx,
			s.pool,
			int(data.Id),
			float64(req.Msg.Start.GetSessionTime()),
			int(req.Msg.Num))

	default:
		err = util.ErrInvalidStartSelector
	}

	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&racestatev1.GetSpeedmapsResponse{
		Speedmaps:       tmp.Data,
		LastTs:          timestamppb.New(tmp.LastTimestamp),
		LastSessionTime: tmp.LastSessionTime,
	}), nil
}

// helper function to store data in the database within a transaction
// function evalates epd.RecordingMode to determine if data should be stored
//
//nolint:whitespace // can't make both editor and linter happy
func (s *stateServer) storeData(
	ctx context.Context,
	epd *utils.EventProcessingData,
	storeFunc func(ctx context.Context, tx pgx.Tx) error,
) error {
	if epd.RecordingMode == providerv1.RecordingMode_RECORDING_MODE_DO_NOT_PERSIST {
		return nil
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := storeFunc(ctx, tx); err != nil {
			return err
		}
		return nil
	})
}
