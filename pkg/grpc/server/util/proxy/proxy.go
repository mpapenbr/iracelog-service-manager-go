package proxy

import (
	"errors"
	"fmt"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	livedatav1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/livedata/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

type (
	EventData struct {
		Event *eventv1.Event
		Track *trackv1.Track
		Owner string
	}
	// PublishProxy is the interface for publishing data from (external) race data
	// providers. These data can be easily converted for response data.
	PublishProxy interface {
		// handles the registration of a new event
		PublishEventRegistered(epd *utils.EventProcessingData) error
		// handles the unregistration of an event
		PublishEventUnregistered(eventKey string) error
		// this method is uses by the instance processing the incoming data
		// the purpose is to distribute the incoming data to all subscribers
		PublishRaceStateData(req *racestatev1.PublishStateRequest) error
		// this method is uses by the instance processing the incoming data
		// the purpose is to distribute the incoming data to all subscribers
		PublishSpeedmapData(req *racestatev1.PublishSpeedmapRequest) error
		// this method is uses by the instance processing the incoming data
		// the purpose is to distribute the incoming data to all subscribers
		PublishDriverData(req *racestatev1.PublishDriverDataRequest) error
	}

	DataProxy interface {
		PublishProxy
		LiveEvents() []*EventData

		// returns the event data for the given selector
		GetEvent(sel *commonv1.EventSelector) (*EventData, error)

		// subscribe to race state data
		// the returned channel is the provider for outgoing live messages
		SubscribeRaceStateData(sel *commonv1.EventSelector) (
			dataChan <-chan *racestatev1.PublishStateRequest,
			quitChan chan<- struct{},
			err error,
		)

		// subscribe to speedmap data
		// the returned channel is the provider for outgoing live messages
		SubscribeSpeedmapData(sel *commonv1.EventSelector) (
			dataChan <-chan *racestatev1.PublishSpeedmapRequest,
			quitChan chan<- struct{},
			err error,
		)

		// subscribe to driver data
		// the returned channel is the provider for outgoing live messages
		SubscribeDriverData(sel *commonv1.EventSelector) (
			dataChan <-chan *livedatav1.LiveDriverDataResponse,
			quitChan chan<- struct{},
			err error,
		)

		// subscribe to analysis data
		// the returned channel is the provider for outgoing live messages
		SubscribeAnalysisData(sel *commonv1.EventSelector) (
			dataChan <-chan *analysisv1.Analysis,
			quitChan chan<- struct{},
			err error,
		)

		// subscribe to snapshot data
		// the returned channel is the provider for outgoing live messages
		SubscribeSnapshotData(sel *commonv1.EventSelector) (
			dataChan <-chan *analysisv1.SnapshotData,
			quitChan chan<- struct{},
			err error,
		)
		// provides all snapshot data for the given event
		HistorySnapshotData(sel *commonv1.EventSelector) []*analysisv1.SnapshotData
		// performs cleanup
		Close()
	}

	EmptyProxy struct{}
)

var ErrEventNotFound = errors.New("event not found")

func (e EmptyProxy) PublishEventRegistered(epd *utils.EventProcessingData) error {
	return fmt.Errorf("PublishEventRegistered not implemented")
}

func (e EmptyProxy) PublishEventUnregistered(eventKey string) error {
	return fmt.Errorf("PublishEventUnregistered not implemented")
}

func (e EmptyProxy) PublishRaceStateData(req *racestatev1.PublishStateRequest) error {
	return fmt.Errorf("PublishRaceStateData not implemented")
}

func (e EmptyProxy) PublishSpeedmapData(req *racestatev1.PublishSpeedmapRequest) error {
	return fmt.Errorf("PublishSpeedmapData not implemented")
}

func (e EmptyProxy) PublishDriverData(req *racestatev1.PublishDriverDataRequest) error {
	return fmt.Errorf("PublishDriverData not implemented")
}

func (e EmptyProxy) GetEvent(sel *commonv1.EventSelector) (*EventData, error) {
	return nil, fmt.Errorf("GetEvent not implemented")
}

//nolint:whitespace // false positive
func (e EmptyProxy) SubscribeRaceStateData(sel *commonv1.EventSelector) (
	d <-chan *racestatev1.PublishStateRequest,
	q chan<- struct{},
	err error,
) {
	return nil, nil, fmt.Errorf("SubscribeRaceStateData not implemented")
}

//nolint:whitespace // false positive
func (e EmptyProxy) SubscribeSpeedmapData(sel *commonv1.EventSelector) (
	d <-chan *racestatev1.PublishSpeedmapRequest,
	q chan<- struct{},
	err error,
) {
	return nil, nil, fmt.Errorf("SubscribeSpeedmapData not implemented")
}

//nolint:whitespace // false positive
func (e EmptyProxy) SubscribeDriverData(sel *commonv1.EventSelector) (
	d <-chan *livedatav1.LiveDriverDataResponse,
	q chan<- struct{},
	err error,
) {
	return nil, nil, fmt.Errorf("SubscribeDriverData not implemented")
}

//nolint:whitespace // false positive
func (e EmptyProxy) SubscribeAnalysisData(sel *commonv1.EventSelector) (
	d <-chan *analysisv1.Analysis,
	q chan<- struct{},
	err error,
) {
	return nil, nil, fmt.Errorf("SubscribeAnalysisData not implemented")
}

//nolint:whitespace // false positive
func (e EmptyProxy) SubscribeSnapshotData(sel *commonv1.EventSelector) (
	d <-chan *analysisv1.SnapshotData,
	q chan<- struct{},
	err error,
) {
	return nil, nil, fmt.Errorf("SubscribeSnapshotData not implemented")
}

//nolint:lll // readablity
func (e EmptyProxy) HistorySnapshotData(sel *commonv1.EventSelector) []*analysisv1.SnapshotData {
	return []*analysisv1.SnapshotData{}
}

func (e EmptyProxy) LiveEvents() []*EventData {
	return nil
}

func (e EmptyProxy) Close() {
}
