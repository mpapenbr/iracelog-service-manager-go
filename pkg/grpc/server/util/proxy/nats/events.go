package nats

import (
	"context"
	"sync"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy"
)

type (
	// GlobalEvents is a global event store that is used to keep track of all events
	// this data is used during initialization of the NatsProxy.
	// the purpse is to enable instances that are started later to be able to deliver
	// data as well.
	GlobalEvents struct {
		kv     jetstream.KeyValue
		events map[string]*proxy.EventData
		mutex  sync.Mutex
		l      *log.Logger
		rev    uint64
	}
)

func NewGlobalEvents(kv jetstream.KeyValue, l *log.Logger) (*GlobalEvents, error) {
	ret := &GlobalEvents{
		kv:     kv,
		mutex:  sync.Mutex{},
		events: make(map[string]*proxy.EventData),
		l:      l,
	}
	if err := ret.setupListener(); err != nil {
		return nil, err
	}
	return ret, nil
}

//nolint:whitespace // editor/linter issue
func (g *GlobalEvents) CurrentLiveEvents() (
	lookup map[string]*proxy.EventData,
	err error,
) {
	var kve jetstream.KeyValueEntry
	if kve, err = g.kv.Get(context.Background(), "events"); err != nil {
		return nil, err
	}
	conv := eventLookupTransfer{}
	if lookup, err = conv.FromBinary(kve.Value()); err == nil {
		return lookup, nil
	} else {
		return nil, err
	}
}

// register watcher on kv store
func (g *GlobalEvents) setupListener() error {
	w, err := g.kv.Watch(context.Background(), "events")
	if err != nil {
		return err
	}
	go func() {
		conv := eventLookupTransfer{}
		for kve := range w.Updates() {
			if kve == nil {
				g.l.Debug("watchEventData nil")
				continue
			}
			g.l.Debug("watchEventData",
				log.Int("value-len", len(kve.Value())),
				log.String("op", kve.Operation().String()),
				log.Uint64("rev", kve.Revision()),
			)
			g.rev = kve.Revision()
			var incomingData map[string]*proxy.EventData
			if incomingData, err = conv.FromBinary(kve.Value()); err == nil {
				g.mutex.Lock()
				g.events = incomingData
				g.mutex.Unlock()
				g.l.Debug("events updated", log.Any("events", incomingData))
			} else {
				g.l.Error("error unmarshalling event data", log.ErrorField(err))
			}
		}
		g.l.Debug("eventData watch done")
	}()
	return nil
}

// called when this instance is recording an event
func (g *GlobalEvents) RegisterEvent(e *proxy.EventData) {
	g.l.Debug("RegisterEvent", log.String("key", e.Event.GetKey()))
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.events[e.Event.GetKey()] = e
	data, err := eventLookupTransfer{}.ToBinary(g.events)
	if err == nil {
		var rev uint64
		if rev, err = g.kv.Update(context.Background(), "events", data, g.rev); err != nil {
			g.l.Error("error writing event data", log.ErrorField(err))
		} else {
			g.l.Debug("event data written", log.Uint64("rev", rev))
			g.rev = rev
		}
	} else {
		g.l.Error("error marshaling event data", log.ErrorField(err))
	}
}

// called on the recording instance when it is no longer recording an event
func (g *GlobalEvents) UnregisterEvent(eventKey string) {
	g.l.Debug("UnregisterEvent", log.String("key", eventKey))
	g.mutex.Lock()
	defer g.mutex.Unlock()
	// remove the event from the map
	delete(g.events, eventKey)

	data, err := eventLookupTransfer{}.ToBinary(g.events)
	if err == nil {
		var rev uint64
		if rev, err = g.kv.Update(context.Background(), "events", data, g.rev); err != nil {
			g.l.Error("error writing event data", log.ErrorField(err))
		} else {
			g.l.Debug("event data written", log.Uint64("rev", rev))
			g.rev = rev
		}
	} else {
		g.l.Error("error marshaling event data", log.ErrorField(err))
	}
}
