package utils

import (
	"context"
	"errors"
	"sync"
	"time"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	providerev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/provider/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"google.golang.org/protobuf/proto"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/car"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/race"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/broadcast"
)

type Option func(*EventLookup)

func WithContext(ctx context.Context) Option {
	return func(el *EventLookup) {
		el.ctx = ctx
	}
}

// if staleDuration > 0, a watchdog will be started to remove stale events
func WithStaleDuration(d time.Duration) Option {
	return func(el *EventLookup) {
		el.staleDuration = d
	}
}

func WithDeleteEventCB(cb func(eventKey string)) Option {
	return func(el *EventLookup) {
		el.deleteEventCB = cb
	}
}

var idCounter int = 1

func NewEventLookup(opts ...Option) *EventLookup {
	ret := &EventLookup{
		lookup: make(map[string]*EventProcessingData),
		ctx:    context.Background(),
		mutex:  sync.Mutex{},
		log:    log.Default().Named("grpc.eventmgr"),
		id:     idCounter,
	}
	idCounter++
	for _, opt := range opts {
		opt(ret)
	}
	if ret.staleDuration > 0 {
		ret.setupWatchdog()
	}

	return ret
}

var ErrEventNotFound = errors.New("event not found")

//nolint:lll // readability
type EventProcessingData struct {
	Event               *eventv1.Event
	Track               *trackv1.Track
	Processor           *processing.Processor
	AnalysisBroadcast   broadcast.BroadcastServer[*analysisv1.Analysis]
	RacestateBroadcast  broadcast.BroadcastServer[*racestatev1.PublishStateRequest]
	DriverDataBroadcast broadcast.BroadcastServer[*racestatev1.PublishDriverDataRequest]
	SpeedmapBroadcast   broadcast.BroadcastServer[*racestatev1.PublishSpeedmapRequest]
	ReplayInfoBroadcast broadcast.BroadcastServer[*eventv1.ReplayInfo]
	SnapshotBroadcast   broadcast.BroadcastServer[*analysisv1.SnapshotData]
	Mutex               sync.Mutex
	LastDriverData      *racestatev1.PublishDriverDataRequest
	LastRaceState       *racestatev1.PublishStateRequest
	LastAnalysisData    *analysisv1.Analysis
	LastReplayInfo      *eventv1.ReplayInfo
	RecordingMode       providerev1.RecordingMode
	LastRsInfoId        int                        // holds the last rs_info_id for storing state data
	LastDataEvent       time.Time                  // holds the time of the last incoming data event
	SnapshotData        []*analysisv1.SnapshotData // holds the snapshot data for current event
	RaceSessions        []uint32                   // holds the ids of race sessions
}
type EventLookup struct {
	lookup        map[string]*EventProcessingData
	ctx           context.Context
	staleDuration time.Duration
	mutex         sync.Mutex
	log           *log.Logger
	deleteEventCB func(eventKey string)
	id            int
}

//nolint:whitespace,funlen,lll // can't make both editor and linter happy, readability
func (e *EventLookup) AddEvent(
	event *eventv1.Event,
	track *trackv1.Track,
	recordingMode providerev1.RecordingMode,
) *EventProcessingData {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if _, ok := e.lookup[event.Key]; ok {
		return nil
	}
	analysisSource := make(chan *analysisv1.Analysis)
	racestateSource := make(chan *racestatev1.PublishStateRequest)
	driverDataSource := make(chan *racestatev1.PublishDriverDataRequest)
	speedmapSource := make(chan *racestatev1.PublishSpeedmapRequest)
	replayInfoSource := make(chan *eventv1.ReplayInfo)
	snapshotSource := make(chan *analysisv1.SnapshotData)

	raceSessions := CollectRaceSessions(event)
	cp := car.NewCarProcessor(
		car.WithRaceSessions(raceSessions),
	)
	rp := race.NewRaceProcessor(
		race.WithCarProcessor(cp),
		race.WithRaceSessions(raceSessions),
	)

	epd := &EventProcessingData{
		Event:         event,
		Track:         track,
		RecordingMode: recordingMode,
		Processor: processing.NewProcessor(
			processing.WithCarProcessor(cp),
			processing.WithRaceProcessor(rp),
			processing.WithPublishChannels(
				analysisSource,
				racestateSource,
				driverDataSource,
				speedmapSource,
				replayInfoSource,
				snapshotSource,
			),
		),

		AnalysisBroadcast:   broadcast.NewBroadcastServer(event.Key, "analysis", analysisSource),
		RacestateBroadcast:  broadcast.NewBroadcastServer(event.Key, "racestate", racestateSource),
		DriverDataBroadcast: broadcast.NewBroadcastServer(event.Key, "driverdata", driverDataSource),
		SpeedmapBroadcast:   broadcast.NewBroadcastServer(event.Key, "speedmap", speedmapSource),
		ReplayInfoBroadcast: broadcast.NewBroadcastServer(event.Key, "replayInfo", replayInfoSource),
		SnapshotBroadcast:   broadcast.NewBroadcastServer(event.Key, "snapshot", snapshotSource),
		Mutex:               sync.Mutex{},
		LastDataEvent:       time.Now(),
		SnapshotData:        make([]*analysisv1.SnapshotData, 0),
		RaceSessions:        raceSessions,
	}
	epd.setupOwnListeners()
	e.lookup[event.Key] = epd
	return epd
}

func (e *EventLookup) SetDeleteEventCB(cb func(eventKey string)) {
	e.deleteEventCB = cb
}

//nolint:whitespace // can't make both editor and linter happy
func (e *EventLookup) GetEvent(selector *commonv1.EventSelector) (
	*EventProcessingData, error,
) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	return e.getEvent(selector)
}

//nolint:whitespace // can't make both editor and linter happy
func (e *EventLookup) getEvent(selector *commonv1.EventSelector) (
	*EventProcessingData, error,
) {
	switch selector.Arg.(type) {
	case *commonv1.EventSelector_Id:
		for _, v := range e.lookup {
			if v.Event.Id == uint32(selector.GetId()) {
				return v, nil
			}
		}
		return nil, ErrEventNotFound
	case *commonv1.EventSelector_Key:
		if ret, ok := e.lookup[selector.GetKey()]; ok {
			return ret, nil
		}
	}
	return nil, ErrEventNotFound
}

func (e *EventLookup) RemoveEvent(selector *commonv1.EventSelector) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if epd, err := e.getEvent(selector); err == nil {
		epd.Close()
		delete(e.lookup, epd.Event.Key)
	}
}

func (e *EventLookup) GetEvents() []*EventProcessingData {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	ret := make([]*EventProcessingData, 0, len(e.lookup))
	for _, v := range e.lookup {
		ret = append(ret, v)
	}
	return ret
}

// removes all events from the lookup
func (e *EventLookup) Clear() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.lookup = make(map[string]*EventProcessingData)
}

// removes all events and signals shutdown to internal services (for example: watchdogs)
func (e *EventLookup) Close() {
	e.Clear()
	e.ctx.Done()
}

// setup watchdog for event processing data
//
//nolint:gocognit //false positive
func (e *EventLookup) setupWatchdog() {
	go func() {
		for {
			// use select to be able to break out of the loop
			select {
			case <-e.ctx.Done():
				e.log.Debug("event lookup watchdog shutting down")
				return
			default:

				time.Sleep(1 * time.Second)

				toRemove := make([]*EventProcessingData, 0)
				for _, epd := range e.lookup {
					if time.Since(epd.LastDataEvent) > e.staleDuration {
						e.log.Info("watchdog detected stale event",
							log.String("event", epd.Event.Key),
							log.Duration("stale", time.Since(epd.LastDataEvent)))
						toRemove = append(toRemove, epd)
					}
				}
				for _, epd := range toRemove {
					epd.Close()
					delete(e.lookup, epd.Event.Key)
					if e.deleteEventCB != nil {
						e.deleteEventCB(epd.Event.Key)
					}
				}
			}
		}
	}()
}

func (epd *EventProcessingData) MarkDataEvent() {
	epd.LastDataEvent = time.Now()
}

func (epd *EventProcessingData) Close() {
	epd.AnalysisBroadcast.Close()
	epd.RacestateBroadcast.Close()
	epd.DriverDataBroadcast.Close()
	epd.SpeedmapBroadcast.Close()
	epd.ReplayInfoBroadcast.Close()
	epd.SnapshotBroadcast.Close()
}

func (epd *EventProcessingData) setupOwnListeners() {
	go func() {
		ch := epd.RacestateBroadcast.Subscribe()
		for data := range ch {
			//nolint:errcheck // by design
			epd.LastRaceState = proto.Clone(data).(*racestatev1.PublishStateRequest)
		}
	}()
	go func() {
		ch := epd.DriverDataBroadcast.Subscribe()
		for data := range ch {
			//nolint:errcheck // by design
			epd.LastDriverData = proto.Clone(data).(*racestatev1.PublishDriverDataRequest)
		}
	}()
	go func() {
		ch := epd.AnalysisBroadcast.Subscribe()
		for data := range ch {
			//nolint:errcheck // by design
			epd.LastAnalysisData = proto.Clone(data).(*analysisv1.Analysis)
		}
	}()
	go func() {
		ch := epd.ReplayInfoBroadcast.Subscribe()
		for data := range ch {
			//nolint:errcheck // by design
			epd.LastReplayInfo = proto.Clone(data).(*eventv1.ReplayInfo)
		}
	}()
	go func() {
		ch := epd.SnapshotBroadcast.Subscribe()
		for data := range ch {
			//nolint:errcheck // by design
			epd.SnapshotData = append(epd.SnapshotData,
				proto.Clone(data).(*analysisv1.SnapshotData))
		}
	}()
}

func CollectRaceSessions(e *eventv1.Event) []uint32 {
	ret := make([]uint32, 0)
	for _, s := range e.Sessions {
		if s.Type == commonv1.SessionType_SESSION_TYPE_RACE {
			ret = append(ret, s.Num)
		}
	}
	return ret
}
