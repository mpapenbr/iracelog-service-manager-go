package livedata

import (
	"context"

	"buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/livedata/v1/livedatav1connect"
	analysisv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/analysis/v1"
	livedatav1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/livedata/v1"
	speedmapv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/speedmap/v1"
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
	log.Debug("Sending live rspeedmap data",
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

//nolint:whitespace,dupl,gocritic // can't make both editor and linter happy
func (s *liveDataServer) LiveCarInfos(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveCarInfosRequest],
	stream *connect.ServerStream[livedatav1.LiveCarInfosResponse],
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
		if err := stream.Send(&livedatav1.LiveCarInfosResponse{
			CarInfos: work.CarInfos,
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
	dataChan := epd.AnalysisBroadcast.Subscribe()
	//nolint:errcheck // by design
	first := proto.Clone(<-dataChan).(*analysisv1.Analysis)
	firstResp := &livedatav1.LiveAnalysisSelResponse{
		Timestamp:        timestamppb.Now(),
		CarInfos:         first.CarInfos,
		CarLaps:          first.CarLaps,
		CarPits:          first.CarPits,
		CarStints:        first.CarStints,
		RaceOrder:        first.RaceOrder,
		RaceGraph:        first.RaceGraph,
		CarComputeStates: first.CarComputeStates,
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
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_CAR_INFOS:
			ret.CarInfos = a.CarInfos
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_CAR_LAPS:
			ret.CarLaps = s.tailedCarlaps(a.CarLaps, sel.CarLapsNumTail)
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_CAR_PITS:
			ret.CarPits = a.CarPits
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_CAR_STINTS:
			ret.CarStints = a.CarStints
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_RACE_ORDER:
			ret.RaceOrder = a.RaceOrder
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_RACE_GRAPH:
			ret.RaceGraph = s.tailedRaceGraph(a.RaceGraph, sel.RaceGraphNumTail)
		case livedatav1.AnalysisComponent_ANALYSIS_COMPONENT_UNSPECIFIED:
			// nothing
		}
	}
	return ret
}

//nolint:whitespace // can't make both editor and linter happy
func (*liveDataServer) tailedCarlaps(
	in []*analysisv1.CarLaps,
	tail uint32,
) []*analysisv1.CarLaps {
	if len(in) < int(tail) {
		return in
	}
	return in[len(in)-int(tail):]
}

//nolint:whitespace // can't make both editor and linter happy
func (*liveDataServer) tailedRaceGraph(
	in []*analysisv1.RaceGraph,
	tail uint32,
) []*analysisv1.RaceGraph {
	ret := make([]*analysisv1.RaceGraph, len(in))
	for i, r := range in {
		work := analysisv1.RaceGraph{LapNo: r.LapNo, CarClass: r.CarClass}
		if len(r.Gaps) < int(tail) {
			work.Gaps = r.Gaps
		} else {
			work.Gaps = r.Gaps[len(r.Gaps)-int(tail):]
		}
		ret[i] = &work
	}
	return ret
}
