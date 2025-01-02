package nats

import (
	"context"
	"fmt"
	"sync"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	providerv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/provider/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/pubsub"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type (
	NatsPubSub struct {
		pubsub.EmptyPubSubData
		ctx            context.Context
		conn           *nats.Conn
		events         map[string]*eventContainer // holds events over all cluster members
		l              *log.Logger
		mutex          sync.Mutex
		onUnregisterCB func(sel *commonv1.EventSelector)
		subRegister    *nats.Subscription
		subUnregister  *nats.Subscription
	}
	Option         func(*NatsPubSub)
	eventContainer struct {
		eventData    *pubsub.EventData
		subRacestate *nats.Subscription
	}
)

func NewNatsPubSub(conn *nats.Conn, opts ...Option) (*NatsPubSub, error) {
	ret := &NatsPubSub{
		conn:   conn,
		ctx:    context.Background(),
		events: make(map[string]*eventContainer),
		l:      log.Default().Named("nats"),
		mutex:  sync.Mutex{},
	}
	for _, opt := range opts {
		opt(ret)
	}
	if err := ret.setupSubscriptions(); err != nil {
		return nil, err
	}
	return ret, nil
}

func WithContext(ctx context.Context) Option {
	return func(n *NatsPubSub) {
		n.ctx = ctx
	}
}

func WithLogger(l *log.Logger) Option {
	return func(n *NatsPubSub) {
		n.l = l
	}
}

func WithOnUnregisterCB(cb func(sel *commonv1.EventSelector)) Option {
	return func(n *NatsPubSub) {
		n.onUnregisterCB = cb
	}
}

func (n *NatsPubSub) Close() {
	n.conn.Close()
}

// this method is called when the watchdog detects a stale event and deletes it
func (n *NatsPubSub) DeleteEventCallback(eventKey string) {
	n.PublishEventUnregistered(eventKey)
}

func (n *NatsPubSub) PublishEventRegistered(ed *pubsub.EventData) error {
	builder := providerv1.RegisterEventResponse_builder{
		Event: ed.Event,
		Track: ed.Track,
	}
	msg := builder.Build()
	data, _ := proto.Marshal(msg)
	return n.conn.Publish("event.registered", data)
}

func (n *NatsPubSub) PublishEventUnregistered(eventKey string) error {
	return n.conn.Publish("event.unregistered", []byte(eventKey))
}

func (n *NatsPubSub) PublishRaceStateData(req *racestatev1.PublishStateRequest) error {
	data, _ := proto.Marshal(req)
	return n.conn.Publish(fmt.Sprintf("racestate.%s", req.Event.GetKey()), data)
}

func (n *NatsPubSub) LiveEvents() []*pubsub.EventData {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	events := make([]*pubsub.EventData, 0, len(n.events))
	for _, event := range n.events {
		events = append(events, event.eventData)
	}
	return events
}

//nolint:whitespace // false positive
func (n *NatsPubSub) GetEvent(selector *commonv1.EventSelector) (
	*pubsub.EventData, error,
) {
	if event, err := n.getEvent(selector); err != nil {
		return nil, err
	} else {
		return event.eventData, nil
	}
}

//nolint:whitespace // false positive
func (n *NatsPubSub) SubscribeRaceStateData(sel *commonv1.EventSelector) (
	<-chan *racestatev1.PublishStateRequest, error,
) {
	if event, err := n.getEvent(sel); err != nil {
		return nil, err
	} else {
		subj := fmt.Sprintf("racestate.%s", event.eventData.Event.Key)
		dataChan := make(chan *racestatev1.PublishStateRequest)
		sub, err := n.conn.Subscribe(subj, func(msg *nats.Msg) {
			var req racestatev1.PublishStateRequest
			if uErr := proto.Unmarshal(msg.Data, &req); uErr != nil {
				n.l.Error("error unmarshalling racestate data", log.ErrorField(uErr))
				return
			}
			n.l.Debug("received racestate data", log.String("eventKey", req.Event.GetKey()))
			dataChan <- &req
		})
		if err != nil {
			return nil, err
		}
		event.subRacestate = sub
		return dataChan, nil
	}
}

//nolint:whitespace // false positive
func (n *NatsPubSub) UnsubscribeRaceStateData(
	sel *commonv1.EventSelector,
	dataChan <-chan *racestatev1.PublishStateRequest,
) {
	if event, err := n.getEvent(sel); err == nil {
		n.unsubscribe(event.subRacestate)
	}
}

func (n *NatsPubSub) unsubscribe(sub *nats.Subscription) {
	if sub != nil {
		if err := sub.Unsubscribe(); err != nil {
			n.l.Debug("error unsubscribing",
				log.String("sub", sub.Subject),
				log.ErrorField(err))
		} else {
			n.l.Debug("unsubscribed",
				log.String("sub", sub.Subject),
			)
		}
	}
}

//nolint:whitespace // false positive
func (n *NatsPubSub) getEvent(selector *commonv1.EventSelector) (
	*eventContainer, error,
) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	switch selector.Arg.(type) {
	case *commonv1.EventSelector_Id:
		for _, v := range n.events {
			if v.eventData.Event.Id == uint32(selector.GetId()) {
				return v, nil
			}
		}
		return nil, pubsub.ErrEventNotFound
	case *commonv1.EventSelector_Key:
		if ret, ok := n.events[selector.GetKey()]; ok {
			return ret, nil
		}
	}
	return nil, pubsub.ErrEventNotFound
}

func (n *NatsPubSub) setupSubscriptions() error {
	var err error
	if n.subRegister, err = n.conn.Subscribe("event.registered", func(msg *nats.Msg) {
		var regData providerv1.RegisterEventResponse
		if uErr := proto.Unmarshal(msg.Data, &regData); uErr != nil {
			n.l.Error("error unmarshalling event.registered", log.ErrorField(uErr))
			return
		}
		n.l.Debug("received event registered", log.String("eventKey", regData.Event.Key))
		n.mutex.Lock()
		defer n.mutex.Unlock()
		n.events[regData.Event.Key] = &eventContainer{
			eventData: &pubsub.EventData{
				Event: regData.Event,
				Track: regData.Track,
			},
		}
	}); err != nil {
		return err
	}
	if n.subUnregister, err = n.conn.Subscribe("event.unregistered", func(msg *nats.Msg) {
		n.l.Debug("received event unregistered", log.String("eventKey", string(msg.Data)))
		n.mutex.Lock()
		defer n.mutex.Unlock()
		if n.onUnregisterCB != nil {
			selector := &commonv1.EventSelector{
				Arg: &commonv1.EventSelector_Key{Key: string(msg.Data)},
			}
			n.onUnregisterCB(selector)
		}
		delete(n.events, string(msg.Data))
	}); err != nil {
		return err
	}
	return nil
}
