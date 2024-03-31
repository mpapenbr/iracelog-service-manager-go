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
			Timestamp: timestamppb.Now(),
			Session:   d.Session,
			Cars:      d.Cars,
			Messages:  d.Messages,
		}); err != nil {
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
			epd.AnalysisBroadcast.CancelSubscription(dataChan)
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
			epd.SpeedmapBroadcast.CancelSubscription(dataChan)
			return err
		}
	}
	log.Debug("LiveSpeedmap stream closed")
	return nil
}
