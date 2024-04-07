package utils

import (
	"errors"
	"sync"

	analysisv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/analysis/v1"
	eventv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"
	trackv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/track/v1"
	"google.golang.org/protobuf/proto"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/car"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/broadcast"
)

func NewEventLookup() *EventLookup {
	return &EventLookup{
		lookup: make(map[string]*EventProcessingData),
	}
}

var ErrEventNotFound = errors.New("event not found")

type EventProcessingData struct {
	Event               *eventv1.Event
	Track               *trackv1.Track
	Processor           *processing.Processor
	AnalysisBroadcast   broadcast.BroadcastServer[*analysisv1.Analysis]
	RacestateBroadcast  broadcast.BroadcastServer[*racestatev1.PublishStateRequest]
	DriverDataBroadcast broadcast.BroadcastServer[*racestatev1.PublishDriverDataRequest]
	SpeedmapBroadcast   broadcast.BroadcastServer[*racestatev1.PublishSpeedmapRequest]
	Mutex               sync.Mutex
	LastDriverData      *racestatev1.PublishDriverDataRequest
}
type EventLookup struct {
	lookup map[string]*EventProcessingData
}

func (e *EventLookup) AddEvent(event *eventv1.Event, track *trackv1.Track) {
	if _, ok := e.lookup[event.Key]; ok {
		return
	}
	analysisSource := make(chan *analysisv1.Analysis)
	racestateSource := make(chan *racestatev1.PublishStateRequest)
	driverDataSource := make(chan *racestatev1.PublishDriverDataRequest)
	speedmapSource := make(chan *racestatev1.PublishSpeedmapRequest)

	cp := car.NewCarProcessor()
	epd := &EventProcessingData{
		Event: event,
		Track: track,
		Processor: processing.NewProcessor(
			processing.WithCarProcessor(cp),
			processing.WithPublishChannels(
				analysisSource,
				racestateSource,
				driverDataSource,
				speedmapSource),
		),
		AnalysisBroadcast:   broadcast.NewBroadcastServer("analysis", analysisSource),
		RacestateBroadcast:  broadcast.NewBroadcastServer("racestate", racestateSource),
		DriverDataBroadcast: broadcast.NewBroadcastServer("driverdata", driverDataSource),
		SpeedmapBroadcast:   broadcast.NewBroadcastServer("speedmap", speedmapSource),
		Mutex:               sync.Mutex{},
	}
	epd.setupOwnListeners()
	e.lookup[event.Key] = epd
}

func (epd *EventProcessingData) setupOwnListeners() {
	go func() {
		ch := epd.DriverDataBroadcast.Subscribe()
		for data := range ch {
			//nolint:errcheck // by design
			epd.LastDriverData = proto.Clone(data).(*racestatev1.PublishDriverDataRequest)
		}
	}()
}

//nolint:whitespace // can't make both editor and linter happy
func (e *EventLookup) GetEvent(selector *eventv1.EventSelector) (
	*EventProcessingData, error,
) {
	switch selector.Arg.(type) {
	case *eventv1.EventSelector_Id:
		for _, v := range e.lookup {
			if v.Event.Id == uint32(selector.GetId()) {
				return v, nil
			}
		}
		return nil, ErrEventNotFound
	case *eventv1.EventSelector_Key:
		if ret, ok := e.lookup[selector.GetKey()]; ok {
			return ret, nil
		}
	}
	return nil, ErrEventNotFound
}

func (e *EventLookup) RemoveEvent(selector *eventv1.EventSelector) {
	if epd, err := e.GetEvent(selector); err == nil {
		epd.AnalysisBroadcast.Close()
		epd.RacestateBroadcast.Close()
		epd.DriverDataBroadcast.Close()
		epd.SpeedmapBroadcast.Close()
		delete(e.lookup, epd.Event.Key)
	}
}

func (e *EventLookup) GetEvents() []*EventProcessingData {
	ret := make([]*EventProcessingData, 0, len(e.lookup))
	for _, v := range e.lookup {
		ret = append(ret, v)
	}
	return ret
}

func (e *EventLookup) Clear() {
	e.lookup = make(map[string]*EventProcessingData)
}
