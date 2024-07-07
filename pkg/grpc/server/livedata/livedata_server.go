package livedata

import (
	"context"
	"sort"

	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/livedata/v1/livedatav1connect"
	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	livedatav1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/livedata/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	speedmapv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/speedmap/v1"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewServer(opts ...Option) *liveDataServer {
	ret := &liveDataServer{}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type Option func(*liveDataServer)

func WithEventLookup(lookup *utils.EventLookup) Option {
	return func(srv *liveDataServer) {
		srv.lookup = lookup
	}
}

func WithDebugWire(arg bool) Option {
	return func(srv *liveDataServer) {
		srv.debugWire = arg
	}
}

type liveDataServer struct {
	livedatav1connect.UnimplementedLiveDataServiceHandler

	lookup    *utils.EventLookup
	debugWire bool // if true, debug events affecting "wire" actions (send/receive)
}

//nolint:whitespace // can't make both editor and linter happy
func (s *liveDataServer) LiveRaceState(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveRaceStateRequest],
	stream *connect.ServerStream[livedatav1.LiveRaceStateResponse],
) error {
	// get the epd
	epd, err := s.lookup.GetEvent(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	log.Debug("Sending live race states",
		log.String("event", epd.Event.Key),
	)
	dataChan := epd.RacestateBroadcast.Subscribe()

	for d := range dataChan {
		if s.debugWire {
			log.Debug("Send racestate data",
				log.String("event", epd.Event.Key))
		}
		if err := stream.Send(&livedatav1.LiveRaceStateResponse{
			Timestamp: d.Timestamp,
			Session:   d.Session,
			Cars:      d.Cars,
			Messages:  d.Messages,
		}); err != nil {
			epd.Mutex.Lock()
			//nolint:gocritic // by design
			defer epd.Mutex.Unlock()
			epd.RacestateBroadcast.CancelSubscription(dataChan)
			return err
		}

	}
	log.Debug("LiveRaceState stream closed")
	return nil
}

//nolint:whitespace,dupl // can't make both editor and linter happy
func (s *liveDataServer) LiveAnalysis(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveAnalysisRequest],
	stream *connect.ServerStream[livedatav1.LiveAnalysisResponse],
) error {
	// get the epd
	epd, err := s.lookup.GetEvent(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	log.Debug("Sending live race states",
		log.String("event", epd.Event.Key),
	)
	dataChan := epd.AnalysisBroadcast.Subscribe()
	for a := range dataChan {
		if s.debugWire {
			log.Debug("Received racestate data",
				log.String("event", epd.Event.Key))
		}
		if err := stream.Send(&livedatav1.LiveAnalysisResponse{
			Timestamp: timestamppb.Now(),
			Analysis:  proto.Clone(a).(*analysisv1.Analysis),
		}); err != nil {
			s.cancelSubscription(epd, dataChan)
			return err
		}
	}
	log.Debug("LiveAnalysis stream closed")
	return nil
}

//nolint:whitespace,dupl // can't make both editor and linter happy
func (s *liveDataServer) LiveSpeedmap(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveSpeedmapRequest],
	stream *connect.ServerStream[livedatav1.LiveSpeedmapResponse],
) error {
	// get the epd
	epd, err := s.lookup.GetEvent(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	log.Debug("Sending live speedmap data",
		log.String("event", epd.Event.Key),
	)
	dataChan := epd.SpeedmapBroadcast.Subscribe()
	for a := range dataChan {
		if s.debugWire {
			log.Debug("Sending speedmap data",
				log.String("event", epd.Event.Key))
		}
		if err := stream.Send(&livedatav1.LiveSpeedmapResponse{
			Timestamp: a.Timestamp,
			Speedmap:  proto.Clone(a.Speedmap).(*speedmapv1.Speedmap),
		}); err != nil {
			epd.Mutex.Lock()
			//nolint:gocritic // by design
			defer epd.Mutex.Unlock()
			epd.SpeedmapBroadcast.CancelSubscription(dataChan)
			return err
		}
	}
	log.Debug("LiveSpeedmap stream closed")
	return nil
}

//nolint:whitespace,dupl,funlen // can't make both editor and linter happy
func (s *liveDataServer) LiveSnapshotData(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveSnapshotDataRequest],
	stream *connect.ServerStream[livedatav1.LiveSnapshotDataResponse],
) error {
	// get the epd
	epd, err := s.lookup.GetEvent(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	log.Debug("Sending live snapshot data",
		log.String("event", epd.Event.Key),
	)
	if req.Msg.StartFrom == livedatav1.SnapshotStartMode_SNAPSHOT_START_MODE_BEGIN {
		log.Debug("sending snapshots from beginning",
			log.Int("snapshots", len(epd.SnapshotData)))
		for _, a := range epd.SnapshotData {
			if s.debugWire {
				log.Debug("Sending snapshot data",
					log.String("event", epd.Event.Key))
			}
			if err := stream.Send(&livedatav1.LiveSnapshotDataResponse{
				Timestamp:    timestamppb.Now(),
				SnapshotData: proto.Clone(a).(*analysisv1.SnapshotData),
			}); err != nil {
				log.Warn("Error sending live snapshot (first)", log.ErrorField(err))
				return err
			}
		}
	}

	dataChan := epd.SnapshotBroadcast.Subscribe()
	for a := range dataChan {
		if s.debugWire {
			log.Debug("Sending snapshot data",
				log.String("event", epd.Event.Key))
		}
		if err := stream.Send(&livedatav1.LiveSnapshotDataResponse{
			Timestamp:    timestamppb.Now(),
			SnapshotData: proto.Clone(a).(*analysisv1.SnapshotData),
		}); err != nil {
			epd.Mutex.Lock()
			//nolint:gocritic // by design
			defer epd.Mutex.Unlock()
			epd.SnapshotBroadcast.CancelSubscription(dataChan)
			return err
		}
	}
	log.Debug("LiveSnapshot stream closed")
	return nil
}

//nolint:whitespace,dupl,funlen // can't make both editor and linter happy
func (s *liveDataServer) LiveDriverData(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveDriverDataRequest],
	stream *connect.ServerStream[livedatav1.LiveDriverDataResponse],
) error {
	// get the epd
	epd, err := s.lookup.GetEvent(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	log.Debug("Sending live driver data",
		log.String("event", epd.Event.Key),
	)

	//nolint:lll // readability
	composeResp := func(a *racestatev1.PublishDriverDataRequest) *livedatav1.LiveDriverDataResponse {
		return &livedatav1.LiveDriverDataResponse{
			Timestamp:      a.Timestamp,
			Entries:        a.Entries,
			Cars:           a.Cars,
			CarClasses:     a.CarClasses,
			SessionTime:    a.SessionTime,
			CurrentDrivers: a.CurrentDrivers,
		}
	}
	if epd.LastDriverData != nil {
		if err := stream.Send(composeResp(epd.LastDriverData)); err != nil {
			log.Warn("Error sending live driver (first)", log.ErrorField(err))
			return err
		}
	}
	dataChan := epd.DriverDataBroadcast.Subscribe()
	for a := range dataChan {
		if s.debugWire {
			log.Debug("Sending driver data",
				log.String("event", epd.Event.Key))
		}
		//nolint:errcheck // by design
		work := proto.Clone(a).(*racestatev1.PublishDriverDataRequest)
		if err := stream.Send(composeResp(work)); err != nil {
			epd.Mutex.Lock()
			//nolint:gocritic // by design
			defer epd.Mutex.Unlock()
			epd.DriverDataBroadcast.CancelSubscription(dataChan)
			return err
		}
	}
	log.Debug("LiveDriverData stream closed")
	return nil
}

//nolint:whitespace,dupl,gocritic // can't make both editor and linter happy
func (s *liveDataServer) LiveCarOccupancies(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveCarOccupanciesRequest],
	stream *connect.ServerStream[livedatav1.LiveCarOccupanciesResponse],
) error {
	// get the epd
	epd, err := s.lookup.GetEvent(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	log.Debug("Sending live carinfo data",
		log.String("event", epd.Event.Key),
	)
	dataChan := epd.AnalysisBroadcast.Subscribe()
	for a := range dataChan {
		if s.debugWire {
			log.Debug("Sending carinfo data",
				log.String("event", epd.Event.Key))
		}
		//nolint:errcheck // by design
		work := proto.Clone(a).(*analysisv1.Analysis)
		if err := stream.Send(&livedatav1.LiveCarOccupanciesResponse{
			CarOccupancies: work.CarOccupancies,
		}); err != nil {
			s.cancelSubscription(epd, dataChan)
			return err
		}
	}
	log.Debug("LiveCarInfo stream closed")
	return nil
}

//nolint:whitespace,dupl,funlen,gocritic // can't make both editor and linter happy
func (s *liveDataServer) LiveAnalysisSel(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveAnalysisSelRequest],
	stream *connect.ServerStream[livedatav1.LiveAnalysisSelResponse],
) error {
	// get the epd
	epd, err := s.lookup.GetEvent(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	log.Debug("Sending live analysis data by selector",
		log.String("event", epd.Event.Key),
	)
	var snapshotsCopy []*analysisv1.SnapshotData
	for _, snapshot := range epd.SnapshotData {
		snapshotsCopy = append(snapshotsCopy,
			proto.Clone(snapshot).(*analysisv1.SnapshotData))
	}
	dataChan := epd.AnalysisBroadcast.Subscribe()
	//nolint:errcheck // by design
	first := proto.Clone(<-dataChan).(*analysisv1.Analysis)
	firstResp := &livedatav1.LiveAnalysisSelResponse{
		Timestamp:        timestamppb.Now(),
		CarOccupancies:   first.CarOccupancies,
		CarLaps:          first.CarLaps,
		CarPits:          first.CarPits,
		CarStints:        first.CarStints,
		RaceOrder:        first.RaceOrder,
		RaceGraph:        first.RaceGraph,
		CarComputeStates: first.CarComputeStates,
		Snapshots:        snapshotsCopy,
	}
	if err := stream.Send(firstResp); err != nil {
		log.Warn("Error sending live analysis data by selector (first)", log.ErrorField(err))
		s.cancelSubscription(epd, dataChan)
		return err
	}
	for a := range dataChan {
		if s.debugWire {
			log.Debug("Sending carinfo data",
				log.String("event", epd.Event.Key))
		}
		//nolint:errcheck // by design
		work := proto.Clone(a).(*analysisv1.Analysis)
		if err := stream.Send(
			s.composeAnalysisResponse(work, req.Msg.Selector)); err != nil {
			log.Warn("Error sending live analysis data by selector", log.ErrorField(err))
			s.cancelSubscription(epd, dataChan)
			return err
		}
	}
	log.Debug("LiveAnalysisSel stream closed")
	return nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *liveDataServer) cancelSubscription(
	epd *utils.EventProcessingData,
	dataChan <-chan *analysisv1.Analysis,
) {
	epd.Mutex.Lock()
	defer epd.Mutex.Unlock()
	epd.AnalysisBroadcast.CancelSubscription(dataChan)
}

//nolint:whitespace // can't make both editor and linter happy
func (s *liveDataServer) composeAnalysisResponse(
	a *analysisv1.Analysis,
	sel *livedatav1.AnalysisSelector,
) *livedatav1.LiveAnalysisSelResponse {
	ret := &livedatav1.LiveAnalysisSelResponse{
		Timestamp: timestamppb.Now(),
	}
	for _, c := range sel.Components {
		switch c {
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_CAR_COMPUTE_STATES:
			ret.CarComputeStates = a.CarComputeStates
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_CAR_OCCUPANCIES:
			ret.CarOccupancies = a.CarOccupancies
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_CAR_LAPS:
			ret.CarLaps = tailedCarlaps(a.CarLaps, sel.CarLapsNumTail)
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_CAR_PITS:
			ret.CarPits = a.CarPits
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_CAR_STINTS:
			ret.CarStints = a.CarStints
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_RACE_ORDER:
			ret.RaceOrder = a.RaceOrder
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_RACE_GRAPH:
			ret.RaceGraph = tailedRaceGraph(a.RaceGraph, sel.RaceGraphNumTail)
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_UNSPECIFIED:
			// nothing
		}
	}
	return ret
}

//nolint:whitespace // can't make both editor and linter happy
func tailedCarlaps(
	in []*analysisv1.CarLaps,
	tail uint32,
) []*analysisv1.CarLaps {
	ret := make([]*analysisv1.CarLaps, len(in))
	for i, r := range in {
		work := analysisv1.CarLaps{CarNum: r.CarNum}
		if len(r.Laps) < int(tail) {
			work.Laps = r.Laps
		} else {
			work.Laps = r.Laps[len(r.Laps)-int(tail):]
		}
		ret[i] = &work
	}
	return ret
}

//nolint:whitespace // can't make both editor and linter happy
func tailedRaceGraph(
	in []*analysisv1.RaceGraph,
	tail uint32,
) []*analysisv1.RaceGraph {
	ret := make([]*analysisv1.RaceGraph, 0)
	source := toMap(in)

	// sort keys
	keys := make([]string, 0, len(source))
	for k := range source {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		m := source[k]
		if len(m) < int(tail) {
			ret = append(ret, m...)
		} else {
			ret = append(ret, m[len(m)-int(tail):]...)
		}
	}

	return ret
}

func toMap(in []*analysisv1.RaceGraph) map[string][]*analysisv1.RaceGraph {
	ret := make(map[string][]*analysisv1.RaceGraph, 0)
	for _, r := range in {
		if _, ok := ret[r.CarClass]; !ok {
			ret[r.CarClass] = make([]*analysisv1.RaceGraph, 0)
		}
		ret[r.CarClass] = append(ret[r.CarClass], r)
	}
	return ret
}
