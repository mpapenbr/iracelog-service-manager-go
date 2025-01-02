package pubsub

// TODO: rename to proxy?
import (
	"errors"
	"fmt"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
)

type (
	EventData struct {
		Event *eventv1.Event
		Track *trackv1.Track
	}
	PublishProxy interface {
		// handles the registration of a new event
		PublishEventRegistered(ed *EventData) error
		// handles the unregistration of an event
		PublishEventUnregistered(eventKey string) error
		// this method is uses by the instance processing the incoming data
		// the purpose is to distribute the incoming data to all subscribers
		PublishRaceStateData(req *racestatev1.PublishStateRequest) error
	}
	// new name: ProxyData DataProxy?
	PubSubData interface {
		PublishProxy
		LiveEvents() []*EventData

		// returns the event data for the given selector
		GetEvent(sel *commonv1.EventSelector) (*EventData, error)

		// subscribe to race state data
		// the returned channel is the provider for outgoing live messages
		SubscribeRaceStateData(sel *commonv1.EventSelector) (
			<-chan *racestatev1.PublishStateRequest, error,
		)
		UnsubscribeRaceStateData(
			sel *commonv1.EventSelector,
			dataChan <-chan *racestatev1.PublishStateRequest)
		// performs cleanup
		Close()
	}

	EmptyPubSubData struct{}
)

var ErrEventNotFound = errors.New("event not found")

func (e EmptyPubSubData) PublishEventRegistered(ed *EventData) error {
	return fmt.Errorf("PublishEventRegistered not implemented")
}

func (e EmptyPubSubData) PublishEventUnregistered(eventKey string) error {
	return fmt.Errorf("PublishEventUnregistered not implemented")
}

func (e EmptyPubSubData) PublishRaceStateData(req *racestatev1.PublishStateRequest) error {
	return fmt.Errorf("PublishRaceStateData not implemented")
}

func (e EmptyPubSubData) GetEvent(sel *commonv1.EventSelector) (*EventData, error) {
	return nil, fmt.Errorf("GetEvent not implemented")
}

//nolint:whitespace // false positive
func (e EmptyPubSubData) SubscribeRaceStateData(sel *commonv1.EventSelector) (
	<-chan *racestatev1.PublishStateRequest, error,
) {
	return nil, fmt.Errorf("SubscribeRaceStateData not implemented")
}

func (e EmptyPubSubData) UnsubscribeRaceStateData(
	sel *commonv1.EventSelector,
	dataChan <-chan *racestatev1.PublishStateRequest) {
}

func (e EmptyPubSubData) LiveEvents() []*EventData {
	return nil
}

func (e EmptyPubSubData) Close() {
}
