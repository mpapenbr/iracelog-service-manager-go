package pubsub

import (
	"errors"
	"fmt"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
)

type (
	EventData struct {
		Event *eventv1.Event
		Track *trackv1.Track
	}
	PubSubData interface {
		// handles the registration of a new event
		PublishEventRegistered(ed *EventData)
		// handles the unregistration of an event
		PublishEventUnregistered(eventKey string)

		LiveEvents() []*EventData

		// returns the event data for the given selector
		GetEvent(sel *commonv1.EventSelector) (*EventData, error)

		// performs cleanup
		Close()
	}

	EmptyPubSubData struct{}
)

var ErrEventNotFound = errors.New("event not found")

func (e EmptyPubSubData) PublishEventRegistered(ed *EventData) {
}

func (e EmptyPubSubData) PublishEventUnregistered(eventKey string) {
}

func (e EmptyPubSubData) GetEvent(sel *commonv1.EventSelector) (*EventData, error) {
	return nil, fmt.Errorf("GetEvent not implemented")
}

func (e EmptyPubSubData) LiveEvents() []*EventData {
	return nil
}

func (e EmptyPubSubData) Close() {
}
