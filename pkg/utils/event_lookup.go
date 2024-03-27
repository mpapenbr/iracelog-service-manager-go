package utils

import (
	"errors"

	eventv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/event/v1"
)

func NewEventLookup() *EventLookup {
	return &EventLookup{
		lookup: make(map[string]*eventv1.Event),
	}
}

var ErrEventNotFound = errors.New("event not found")

type EventLookup struct {
	lookup map[string]*eventv1.Event
}

func (e *EventLookup) AddEvent(event *eventv1.Event) {
	if _, ok := e.lookup[event.Key]; ok {
		return
	}
	e.lookup[event.Key] = event
}

//nolint:whitespace // can't make both editor and linter happy
func (e *EventLookup) GetEvent(selector *eventv1.EventSelector) (
	*eventv1.Event, error,
) {
	switch selector.Arg.(type) {
	case *eventv1.EventSelector_Id:
		for _, v := range e.lookup {
			if v.Id == uint32(selector.GetId()) {
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
	if event, err := e.GetEvent(selector); err == nil {
		delete(e.lookup, event.Key)
	}
}

func (e *EventLookup) GetEvents() []*eventv1.Event {
	ret := make([]*eventv1.Event, 0, len(e.lookup))
	for _, v := range e.lookup {
		ret = append(ret, v)
	}
	return ret
}

func (e *EventLookup) Clear() {
	e.lookup = make(map[string]*eventv1.Event)
}
