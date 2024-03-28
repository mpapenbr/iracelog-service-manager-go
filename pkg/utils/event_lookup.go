package utils

import (
	"errors"

	eventv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/event/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/car"
)

func NewEventLookup() *EventLookup {
	return &EventLookup{
		lookup: make(map[string]*EventProcessingData),
	}
}

var ErrEventNotFound = errors.New("event not found")

type EventProcessingData struct {
	Event     *eventv1.Event
	Processor *processing.Processor
}
type EventLookup struct {
	lookup map[string]*EventProcessingData
}

func (e *EventLookup) AddEvent(event *eventv1.Event) {
	if _, ok := e.lookup[event.Key]; ok {
		return
	}
	cp := car.NewCarProcessor()
	epd := &EventProcessingData{
		Event: event,
		Processor: processing.NewProcessor(
			processing.WithCarProcessor(cp),
		),
	}
	e.lookup[event.Key] = epd
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
		delete(e.lookup, epd.Event.Key)
	}
}

func (e *EventLookup) GetEvents() []*eventv1.Event {
	ret := make([]*eventv1.Event, 0, len(e.lookup))
	for _, v := range e.lookup {
		ret = append(ret, v.Event)
	}
	return ret
}

func (e *EventLookup) Clear() {
	e.lookup = make(map[string]*EventProcessingData)
}
