package local

import (
	"sync"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	containerv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/container/v1"
	livedatav1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/livedata/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy/helper"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

// DataProxy implementation based on local EventLookup
type (
	LocalProxy struct {
		proxy.EmptyProxy
		lookup *utils.EventLookup
		l      *log.Logger
		mutex  sync.Mutex
	}
	Option func(*LocalProxy)
)

// NewLocalProxy creates a new LocalPubSub
func NewLocalProxy(lookup *utils.EventLookup, opts ...Option) *LocalProxy {
	ret := &LocalProxy{
		lookup: lookup,
		l:      log.Default().Named("grpc.proxy.local"),
		mutex:  sync.Mutex{},
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func WithLogger(arg *log.Logger) Option {
	return func(l *LocalProxy) {
		l.l = arg
	}
}

func (l *LocalProxy) PublishEventRegistered(epd *utils.EventProcessingData) error {
	return nil
}

func (l *LocalProxy) PublishEventUnregistered(eventKey string) error {
	return nil
}

func (l *LocalProxy) PublishRaceStateData(req *racestatev1.PublishStateRequest) error {
	return nil
}

//nolint:lll // readablity
func (l *LocalProxy) PublishSpeedmapData(req *racestatev1.PublishSpeedmapRequest) error {
	return nil
}

//nolint:lll // readablity
func (l *LocalProxy) PublishDriverData(req *racestatev1.PublishDriverDataRequest) error {
	return nil
}

// this method is called when the watchdog detects a stale event and deletes it
//
//nolint:errcheck // by design
func (l *LocalProxy) DeleteEventCallback(eventKey string) {
	l.l.Debug("DeleteEventCallback", log.String("eventKey", eventKey))
}

func (l *LocalProxy) LiveEvents() []*containerv1.EventContainer {
	currentEvents := l.lookup.GetEvents()
	ret := make([]*containerv1.EventContainer, 0, len(currentEvents))
	for _, v := range currentEvents {
		ret = append(ret, &containerv1.EventContainer{
			Event: v.Event,
			Track: v.Track,
			Owner: v.Owner,
		})
	}
	return ret
}

//nolint:whitespace // editor/linter issue
func (l *LocalProxy) GetEvent(sel *commonv1.EventSelector) (
	*containerv1.EventContainer,
	error,
) {
	epd, err := l.lookup.GetEvent(sel)
	if err != nil {
		return nil, err
	}
	return &containerv1.EventContainer{
		Event: epd.Event,
		Track: epd.Track,
		Owner: epd.Owner,
	}, nil
}

//nolint:whitespace,dupl // false positive
func (l *LocalProxy) SubscribeRaceStateData(sel *commonv1.EventSelector) (
	a <-chan *racestatev1.PublishStateRequest,
	b chan<- struct{},
	e error,
) {
	epd, err := l.lookup.GetEvent(sel)
	if err != nil {
		return nil, nil, err
	}
	eventKey := epd.Event.GetKey()
	sourceChan := epd.RacestateBroadcast.Subscribe()
	quitChan := make(chan struct{})

	go func() {
		l.l.Debug("raceStateData waiting on quitChan", log.String("eventKey", eventKey))
		<-quitChan
		l.l.Debug("raceStateData quitChan was closed", log.String("eventKey", eventKey))
		epd.RacestateBroadcast.CancelSubscription(sourceChan)
	}()

	return sourceChan, quitChan, nil
}

//nolint:whitespace,dupl // false positive
func (l *LocalProxy) SubscribeSpeedmapData(sel *commonv1.EventSelector) (
	d <-chan *racestatev1.PublishSpeedmapRequest,
	q chan<- struct{},
	err error,
) {
	epd, err := l.lookup.GetEvent(sel)
	if err != nil {
		return nil, nil, err
	}
	eventKey := epd.Event.GetKey()
	sourceChan := epd.SpeedmapBroadcast.Subscribe()
	quitChan := make(chan struct{})

	go func() {
		l.l.Debug("speedmapData waiting on quitChan", log.String("eventKey", eventKey))
		<-quitChan
		l.l.Debug("speedmapData quitChan was closed", log.String("eventKey", eventKey))
		epd.SpeedmapBroadcast.CancelSubscription(sourceChan)
	}()

	return sourceChan, quitChan, nil
}

//nolint:whitespace // false positive
func (l *LocalProxy) SubscribeDriverData(sel *commonv1.EventSelector) (
	d <-chan *livedatav1.LiveDriverDataResponse,
	q chan<- struct{},
	err error,
) {
	epd, err := l.lookup.GetEvent(sel)
	if err != nil {
		return nil, nil, err
	}

	dataChan := make(chan *livedatav1.LiveDriverDataResponse)
	quitChan := make(chan struct{})
	if epd.LastDriverData != nil {
		go func() {
			dataChan <- helper.ComposeLiveDriverDataResponse(epd.LastDriverData)
		}()
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	sourceChan := epd.DriverDataBroadcast.Subscribe()
	eventKey := epd.Event.GetKey()

	go func() {
		for data := range sourceChan {
			dataChan <- helper.ComposeLiveDriverDataResponse(data)
		}
		l.l.Debug("SubscribeDriverData: done", log.String("event", epd.Event.GetKey()))
		close(dataChan)
	}()
	go func() {
		l.l.Debug("driverData waiting on quitChan", log.String("eventKey", eventKey))
		<-quitChan
		l.l.Debug("driverData quitChan was closed", log.String("eventKey", eventKey))
		epd.DriverDataBroadcast.CancelSubscription(sourceChan)
	}()

	return dataChan, quitChan, nil
}

//nolint:whitespace,dupl // false positive
func (l *LocalProxy) SubscribeAnalysisData(sel *commonv1.EventSelector) (
	d <-chan *analysisv1.Analysis,
	q chan<- struct{},
	err error,
) {
	epd, err := l.lookup.GetEvent(sel)
	if err != nil {
		return nil, nil, err
	}
	eventKey := epd.Event.GetKey()
	sourceChan := epd.AnalysisBroadcast.Subscribe()
	quitChan := make(chan struct{})

	go func() {
		l.l.Debug("analysisData waiting on quitChan", log.String("eventKey", eventKey))
		<-quitChan
		l.l.Debug("analysisData quitChan was closed", log.String("eventKey", eventKey))
		epd.AnalysisBroadcast.CancelSubscription(sourceChan)
	}()

	return sourceChan, quitChan, nil
}

//nolint:whitespace,dupl // false positive
func (l *LocalProxy) SubscribeSnapshotData(sel *commonv1.EventSelector) (
	d <-chan *analysisv1.SnapshotData,
	q chan<- struct{},
	err error,
) {
	epd, err := l.lookup.GetEvent(sel)
	if err != nil {
		return nil, nil, err
	}
	eventKey := epd.Event.GetKey()
	sourceChan := epd.SnapshotBroadcast.Subscribe()
	quitChan := make(chan struct{})

	go func() {
		l.l.Debug("snapshotData waiting on quitChan", log.String("eventKey", eventKey))
		<-quitChan
		l.l.Debug("snapshotData quitChan was closed", log.String("eventKey", eventKey))
		epd.SnapshotBroadcast.CancelSubscription(sourceChan)
	}()

	return sourceChan, quitChan, nil
}

//nolint:lll // readablity
func (l *LocalProxy) HistorySnapshotData(sel *commonv1.EventSelector) []*analysisv1.SnapshotData {
	epd, err := l.lookup.GetEvent(sel)
	if err != nil {
		return nil
	}
	return epd.SnapshotData
}
