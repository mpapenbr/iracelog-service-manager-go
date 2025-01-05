package livedata

import (
	"context"
	"errors"
	"sort"

	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/livedata/v1/livedatav1connect"
	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	livedatav1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/livedata/v1"
	speedmapv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/speedmap/v1"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewServer(opts ...Option) *liveDataServer {
	ret := &liveDataServer{
		log:     log.Default().Named("grpc.live"),
		wireLog: log.Default().Named("grpc.live.wire"),
	}
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

func WithDataProxy(dataProxy proxy.DataProxy) Option {
	return func(srv *liveDataServer) {
		srv.dataProxy = dataProxy
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
	dataProxy proxy.DataProxy
	log       *log.Logger
	wireLog   *log.Logger
	debugWire bool // if true, debug events affecting "wire" actions (send/receive)
}

//nolint:whitespace // can't make both editor and linter happy
func (s *liveDataServer) LiveRaceState(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveRaceStateRequest],
	stream *connect.ServerStream[livedatav1.LiveRaceStateResponse],
) error {
	// get channels
	dataChan, quitChan, err := s.dataProxy.SubscribeRaceStateData(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	eventKey := req.Msg.Event.GetKey()
	s.log.Debug("Sending live race states",
		log.String("event", eventKey),
	)
	go func() {
		<-ctx.Done()
		s.log.Debug("LiveRaceState stream canceled", log.String("event", eventKey))
		close(quitChan)
	}()

	for d := range dataChan {
		if s.debugWire {
			s.wireLog.Debug("Send racestate data",
				log.String("event", eventKey))
		}
		if err := stream.Send(&livedatav1.LiveRaceStateResponse{
			Timestamp: d.Timestamp,
			Session:   d.Session,
			Cars:      d.Cars,
			Messages:  d.Messages,
		}); err != nil {
			s.log.Debug("Error sending LiveRaceState stream", log.String("event", eventKey))
			return err
		}

	}
	s.log.Debug("LiveRaceState source channel closed", log.String("event", eventKey))

	return nil
}

//nolint:whitespace,dupl,errcheck // can't make both editor and linter happy
func (s *liveDataServer) LiveAnalysis(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveAnalysisRequest],
	stream *connect.ServerStream[livedatav1.LiveAnalysisResponse],
) error {
	dataChan, quitChan, err := s.dataProxy.SubscribeAnalysisData(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	eventKey := req.Msg.Event.GetKey()
	s.log.Debug("Sending live analysis data",
		log.String("event", eventKey),
	)
	go func() {
		<-ctx.Done()
		s.log.Debug("LiveAnalysis stream canceled", log.String("event", eventKey))
		close(quitChan)
	}()

	for a := range dataChan {
		if s.debugWire {
			s.wireLog.Debug("Received analysis data",
				log.String("event", eventKey))
		}
		if err := stream.Send(&livedatav1.LiveAnalysisResponse{
			Timestamp: timestamppb.Now(),
			Analysis:  proto.Clone(a).(*analysisv1.Analysis),
		}); err != nil {
			s.log.Debug("Error sending AnalysisData stream", log.String("event", eventKey))
			return err
		}
	}
	s.log.Debug("AnalysisData source channel closed", log.String("event", eventKey))

	return nil
}

//nolint:whitespace,dupl,errcheck // can't make both editor and linter happy
func (s *liveDataServer) LiveSpeedmap(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveSpeedmapRequest],
	stream *connect.ServerStream[livedatav1.LiveSpeedmapResponse],
) error {
	dataChan, quitChan, err := s.dataProxy.SubscribeSpeedmapData(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	eventKey := req.Msg.Event.GetKey()
	s.log.Debug("Sending live speedmap data",
		log.String("event", eventKey),
	)
	go func() {
		<-ctx.Done()
		s.log.Debug("LiveSpeedmap stream canceled", log.String("event", eventKey))
		close(quitChan)
	}()
	for a := range dataChan {
		if s.debugWire {
			s.wireLog.Debug("Sending speedmap data",
				log.String("event", eventKey))
		}
		if err := stream.Send(&livedatav1.LiveSpeedmapResponse{
			Timestamp: a.Timestamp,
			Speedmap:  proto.Clone(a.Speedmap).(*speedmapv1.Speedmap),
		}); err != nil {
			s.log.Debug("Error sending LiveSpeedmap stream", log.String("event", eventKey))
			return err
		}
	}
	s.log.Debug("LiveSpeedmap source channel closed", log.String("event", eventKey))

	return nil
}

//nolint:whitespace,dupl,funlen,errcheck // can't make both editor and linter happy
func (s *liveDataServer) LiveSnapshotData(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveSnapshotDataRequest],
	stream *connect.ServerStream[livedatav1.LiveSnapshotDataResponse],
) error {
	// get channels
	dataChan, quitChan, err := s.dataProxy.SubscribeSnapshotData(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	eventKey := req.Msg.Event.GetKey()
	s.log.Debug("Sending live snapshot data",
		log.String("event", eventKey),
	)
	go func() {
		<-ctx.Done()
		s.log.Debug("LiveSnapshot stream canceled", log.String("event", eventKey))
		close(quitChan)
	}()

	if req.Msg.StartFrom == livedatav1.SnapshotStartMode_SNAPSHOT_START_MODE_BEGIN {
		historyData := s.dataProxy.HistorySnapshotData(req.Msg.Event)
		s.log.Debug("sending snapshots from beginning",
			log.Int("snapshots", len(historyData)))
		for _, a := range historyData {
			if s.debugWire {
				s.wireLog.Debug("Sending snapshot data",
					log.String("event", eventKey))
			}
			if err := stream.Send(&livedatav1.LiveSnapshotDataResponse{
				Timestamp:    timestamppb.Now(),
				SnapshotData: proto.Clone(a).(*analysisv1.SnapshotData),
			}); err != nil {
				s.log.Warn("Error sending live snapshot (first)", log.ErrorField(err))
				return err
			}
		}
	}

	for a := range dataChan {
		if s.debugWire {
			s.wireLog.Debug("Sending snapshot data",
				log.String("event", eventKey))
		}
		if err := stream.Send(&livedatav1.LiveSnapshotDataResponse{
			Timestamp:    timestamppb.Now(),
			SnapshotData: proto.Clone(a).(*analysisv1.SnapshotData),
		}); err != nil {
			s.log.Debug("Error sending LiveSnapshot stream", log.String("event", eventKey))
			return err
		}
	}
	s.log.Debug("LiveSnapshot stream closed")
	return nil
}

//nolint:whitespace,dupl,funlen // can't make both editor and linter happy
func (s *liveDataServer) LiveDriverData(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveDriverDataRequest],
	stream *connect.ServerStream[livedatav1.LiveDriverDataResponse],
) error {
	dataChan, quitChan, err := s.dataProxy.SubscribeDriverData(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	eventKey := req.Msg.Event.GetKey()
	s.log.Debug("Sending live driver data",
		log.String("event", eventKey),
	)

	go func() {
		<-ctx.Done()
		s.log.Debug("LiveDriverData stream canceled", log.String("event", eventKey))
		close(quitChan)
	}()

	for a := range dataChan {
		if s.debugWire {
			s.wireLog.Debug("Sending driver data",
				log.String("event", eventKey))
		}
		//nolint:errcheck // by design
		work := proto.Clone(a).(*livedatav1.LiveDriverDataResponse)
		if err := stream.Send(work); err != nil {
			s.log.Debug("Error sending LiveDriverData stream", log.String("event", eventKey))
			close(quitChan)
			return err
		}
	}
	s.log.Debug("LiveDriverData source stream closed", log.String("event", eventKey))
	return nil
}

//nolint:whitespace,dupl,gocritic // can't make both editor and linter happy
func (s *liveDataServer) LiveCarOccupancies(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveCarOccupanciesRequest],
	stream *connect.ServerStream[livedatav1.LiveCarOccupanciesResponse],
) error {
	// get channels
	dataChan, quitChan, err := s.dataProxy.SubscribeAnalysisData(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	eventKey := req.Msg.Event.GetKey()
	s.log.Debug("Sending car occupancy data",
		log.String("event", eventKey),
	)
	go func() {
		<-ctx.Done()
		s.log.Debug("LiveCarOccupancy stream canceled", log.String("event", eventKey))
		close(quitChan)
	}()
	for a := range dataChan {
		if s.debugWire {
			s.wireLog.Debug("Sending carOccupancy data",
				log.String("event", eventKey))
		}
		//nolint:errcheck // by design
		work := proto.Clone(a).(*analysisv1.Analysis)
		if err := stream.Send(&livedatav1.LiveCarOccupanciesResponse{
			CarOccupancies: work.CarOccupancies,
		}); err != nil {
			s.log.Warn("Error sending carOccupancy data", log.ErrorField(err))
			return err
		}
	}
	s.log.Debug("LiveCarOccupancy stream closed")
	return nil
}

//nolint:whitespace,dupl,funlen,gocritic,errcheck // by design
func (s *liveDataServer) LiveAnalysisSel(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveAnalysisSelRequest],
	stream *connect.ServerStream[livedatav1.LiveAnalysisSelResponse],
) error {
	// get channels
	dataChan, quitChan, err := s.dataProxy.SubscribeAnalysisData(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	eventKey := req.Msg.Event.GetKey()
	s.log.Debug("Sending selected live analysis data",
		log.String("event", eventKey),
	)
	go func() {
		<-ctx.Done()
		s.log.Debug("LiveAnalysisSel stream canceled", log.String("event", eventKey))
		close(quitChan)
	}()

	var snapshotsCopy []*analysisv1.SnapshotData

	for _, snapshot := range s.dataProxy.HistorySnapshotData(req.Msg.Event) {
		snapshotsCopy = append(snapshotsCopy,
			proto.Clone(snapshot).(*analysisv1.SnapshotData))
	}

	//nolint:errcheck // by design
	first := proto.Clone(<-dataChan).(*analysisv1.Analysis)
	if first == nil {
		close(quitChan)
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("no analysis data"))

	}
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
		s.log.Warn("Error sending live analysis data by selector (first)",
			log.ErrorField(err))
		close(quitChan)
		return err
	}
	for a := range dataChan {
		if s.debugWire {
			s.wireLog.Debug("Sending carinfo data",
				log.String("event", eventKey))
		}
		//nolint:errcheck // by design
		work := proto.Clone(a).(*analysisv1.Analysis)
		if err := stream.Send(
			s.composeAnalysisResponse(work, req.Msg.Selector)); err != nil {
			s.log.Warn("Error sending live analysis data by selector", log.ErrorField(err))
			close(quitChan)
			return err
		}
	}
	s.log.Debug("LiveAnalysisSel stream closed")
	return nil
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
