package nats

import (
	"context"
	"fmt"
	"sync"
	"time"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	livedatav1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/livedata/v1"
	providerv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/provider/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy/helper"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/broadcast"
)

type (
	NatsProxy struct {
		proxy.EmptyProxy
		ctx  context.Context
		conn *nats.Conn
		// holds events over all cluster members
		events map[string]*eventContainer
		// holds events for the local cluster member
		localEvents    map[string]*localDataProvider
		l              *log.Logger
		mutex          sync.Mutex
		onUnregisterCB func(sel *commonv1.EventSelector)
		subRegister    *nats.Subscription
		subUnregister  *nats.Subscription
		kv             jetstream.KeyValue
	}
	Option         func(*NatsProxy)
	eventContainer struct {
		eventData            *proxy.EventData
		watchDriverData      jetstream.KeyWatcher
		analysisBroadcaster  *broadcastData[*analysisv1.Analysis]
		racestateBroadcaster *broadcastData[*racestatev1.PublishStateRequest]
		speedmapBroadcaster  *broadcastData[*racestatev1.PublishSpeedmapRequest]
		snapshotBroadcaster  *broadcastData[*analysisv1.SnapshotData]
	}
	broadcastData[T any] struct {
		bs       broadcast.BroadcastServer[T]
		quitChan chan struct{}
		name     string
	}
	localDataProvider struct {
		epd          *utils.EventProcessingData
		analysisChan <-chan *analysisv1.Analysis
		snapshotChan <-chan *analysisv1.SnapshotData
	}
)

func NewNatsProxy(conn *nats.Conn, opts ...Option) (*NatsProxy, error) {
	ret := &NatsProxy{
		conn:        conn,
		ctx:         context.Background(),
		events:      make(map[string]*eventContainer),
		localEvents: make(map[string]*localDataProvider),
		l:           log.Default().Named("nats"),
		mutex:       sync.Mutex{},
	}
	for _, opt := range opts {
		opt(ret)
	}
	if err := ret.setupSubscriptions(); err != nil {
		return nil, err
	}
	if err := ret.setupKV(); err != nil {
		return nil, err
	}
	return ret, nil
}

func WithContext(ctx context.Context) Option {
	return func(n *NatsProxy) {
		n.ctx = ctx
	}
}

func WithLogger(l *log.Logger) Option {
	return func(n *NatsProxy) {
		n.l = l
	}
}

func (n *NatsProxy) Close() {
	n.conn.Close()
}

// this method is called when the watchdog detects a stale event and deletes it
//
//nolint:errcheck // by design
func (n *NatsProxy) DeleteEventCallback(eventKey string) {
	n.PublishEventUnregistered(eventKey)
}

func (n *NatsProxy) SetOnUnregisterCB(cb func(sel *commonv1.EventSelector)) {
	n.onUnregisterCB = cb
}

//nolint:funlen // by design
func (n *NatsProxy) PublishEventRegistered(epd *utils.EventProcessingData) error {
	builder := providerv1.RegisterEventResponse_builder{
		Event: epd.Event,
		Track: epd.Track,
	}
	msg := builder.Build()
	data, _ := proto.Marshal(msg)
	n.mutex.Lock()
	defer n.mutex.Unlock()
	ldp := &localDataProvider{
		epd:          epd,
		analysisChan: epd.AnalysisBroadcast.Subscribe(),
		snapshotChan: epd.SnapshotBroadcast.Subscribe(),
	}

	n.localEvents[epd.Event.Key] = ldp
	go func() {
		for a := range ldp.analysisChan {
			sendData, _ := proto.Marshal(a)
			//nolint:errcheck // by design
			n.conn.Publish(fmt.Sprintf("analysis.%s", epd.Event.GetKey()), sendData)
		}
		n.l.Debug("analysis channel closed", log.String("eventKey", epd.Event.GetKey()))
	}()
	go func() {
		history := epd.SnapshotData
		conv := snapHist{}
		pushHistory := func() {
			if histData, hErr := conv.ToBinary(history); hErr == nil {
				rev, err := n.kv.Put(
					context.Background(),
					fmt.Sprintf("snapshots.%s", epd.Event.GetKey()),
					histData)
				n.l.Debug("snapshots put",
					log.String("key",
						fmt.Sprintf("snapshots.%s", epd.Event.GetKey())),
					log.Int("num", len(history)),
					log.Int("dataLen", len(histData)),
					log.ErrorField(err), log.Uint64("rev", rev))
			} else {
				n.l.Error("error converting snapshot history", log.ErrorField(hErr))
			}
		}
		pushHistory()
		for a := range ldp.snapshotChan {
			sendData, _ := proto.Marshal(a)
			//nolint:errcheck // by design
			n.conn.Publish(fmt.Sprintf("snapshot.%s", epd.Event.GetKey()), sendData)
			history = append(history, a)
			pushHistory()
		}
		n.l.Debug("snapshot channel closed", log.String("eventKey", epd.Event.GetKey()))
	}()

	return n.conn.Publish("event.registered", data)
}

func (n *NatsProxy) PublishEventUnregistered(eventKey string) error {
	return n.conn.Publish("event.unregistered", []byte(eventKey))
}

func (n *NatsProxy) PublishRaceStateData(req *racestatev1.PublishStateRequest) error {
	data, _ := proto.Marshal(req)
	return n.conn.Publish(fmt.Sprintf("racestate.%s", req.Event.GetKey()), data)
}

func (n *NatsProxy) PublishSpeedmapData(req *racestatev1.PublishSpeedmapRequest) error {
	data, _ := proto.Marshal(req)
	return n.conn.Publish(fmt.Sprintf("speedmap.%s", req.Event.GetKey()), data)
}

func (n *NatsProxy) PublishDriverData(req *racestatev1.PublishDriverDataRequest) error {
	data, _ := proto.Marshal(helper.ComposeLiveDriverDataResponse(req))
	rev, err := n.kv.Put(
		context.Background(),
		fmt.Sprintf("driverdata.%s", req.Event.GetKey()),
		data)
	n.l.Debug("driverdata put",
		log.String("key",
			fmt.Sprintf("driverdata.%s", req.Event.GetKey())),
		log.ErrorField(err), log.Uint64("rev", rev))
	return err
}

func (n *NatsProxy) PublishAnalysisData(eventKey string, a *analysisv1.Analysis) error {
	data, _ := proto.Marshal(a)
	return n.conn.Publish(fmt.Sprintf("analysis.%s", eventKey), data)
}

func (n *NatsProxy) LiveEvents() []*proxy.EventData {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	events := make([]*proxy.EventData, 0, len(n.events))
	for _, event := range n.events {
		events = append(events, event.eventData)
	}
	return events
}

//nolint:whitespace // false positive
func (n *NatsProxy) GetEvent(selector *commonv1.EventSelector) (
	*proxy.EventData, error,
) {
	if event, err := n.getEvent(selector); err != nil {
		return nil, err
	} else {
		return event.eventData, nil
	}
}

//nolint:whitespace,dupl,gocritic // false positive
func (n *NatsProxy) SubscribeRaceStateData(sel *commonv1.EventSelector) (
	<-chan *racestatev1.PublishStateRequest,
	chan<- struct{},
	error,
) {
	if event, err := n.getEvent(sel); err != nil {
		return nil, nil, err
	} else {
		dataChan := n.getRacestateBroadcaster(event.eventData.Event.GetKey()).Subscribe()
		quitChan := make(chan struct{})
		eventKey := event.eventData.Event.GetKey()

		go func() {
			n.l.Debug("racestate waiting on quitChan", log.String("eventKey", eventKey))
			<-quitChan
			n.l.Debug("racestate quitChan was closed", log.String("eventKey", eventKey))
			// the broadcaster may be already closed if the event was unregistered
			if bs := n.getRacestateBroadcaster(eventKey); bs != nil {
				bs.CancelSubscription(dataChan)
			}
		}()
		return dataChan, quitChan, nil
	}
}

//nolint:whitespace,dupl,gocritic // false positive
func (n *NatsProxy) SubscribeSpeedmapData(sel *commonv1.EventSelector) (
	<-chan *racestatev1.PublishSpeedmapRequest,
	chan<- struct{},
	error,
) {
	if event, err := n.getEvent(sel); err != nil {
		return nil, nil, err
	} else {
		dataChan := n.getSpeedmapBroadcaster(event.eventData.Event.GetKey()).Subscribe()
		quitChan := make(chan struct{})
		eventKey := event.eventData.Event.GetKey()

		go func() {
			n.l.Debug("speedmap waiting on quitChan", log.String("eventKey", eventKey))
			<-quitChan
			n.l.Debug("speedmap quitChan was closed", log.String("eventKey", eventKey))
			// the broadcaster may be already closed if the event was unregistered
			if bs := n.getSpeedmapBroadcaster(eventKey); bs != nil {
				bs.CancelSubscription(dataChan)
			}
		}()
		return dataChan, quitChan, nil
	}
}

//nolint:whitespace,dupl,funlen // false positive
func (n *NatsProxy) SubscribeDriverData(sel *commonv1.EventSelector) (
	d <-chan *livedatav1.LiveDriverDataResponse,
	q chan<- struct{},
	err error,
) {
	var event *eventContainer
	if event, err = n.getEvent(sel); err != nil {
		return nil, nil, err
	}
	subj := fmt.Sprintf("driverdata.%s", event.eventData.Event.GetKey())
	dataChan := make(chan *livedatav1.LiveDriverDataResponse)
	quitChan := make(chan struct{})
	eventKey := event.eventData.Event.GetKey()
	w, err := n.kv.Watch(context.Background(), subj)
	if err != nil {
		return nil, nil, err
	}
	go func() {
		for kve := range w.Updates() {
			if kve == nil {
				n.l.Debug("watchData nil")
				continue
			}
			var req livedatav1.LiveDriverDataResponse
			n.l.Debug("watchData",
				log.String("key", fmt.Sprintf("driverdata.%s", eventKey)),
				log.Int("value-len", len(kve.Value())),
				log.String("op", kve.Operation().String()),
				log.Uint64("rev", kve.Revision()),
			)
			if uErr := proto.Unmarshal(kve.Value(), &req); uErr != nil {
				n.l.Error("error unmarshalling driverdata", log.ErrorField(uErr))
			}
			n.l.Debug("received driverdata",
				log.String("eventKey", eventKey))
			dataChan <- &req
		}
		n.l.Debug("driverdata watch done",
			log.String("eventKey", eventKey))
	}()
	go func() {
		n.l.Debug("driverData waiting on quitChan", log.String("eventKey", eventKey))
		<-quitChan
		n.l.Debug("driverData quitChan was closed", log.String("eventKey", eventKey))
		n.stopWatch(eventKey, w)
	}()

	return dataChan, quitChan, nil
}

//nolint:whitespace,dupl // false positive
func (n *NatsProxy) SubscribeAnalysisData(sel *commonv1.EventSelector) (
	d <-chan *analysisv1.Analysis,
	q chan<- struct{},
	err error,
) {
	var event *eventContainer
	if event, err = n.getEvent(sel); err != nil {
		return nil, nil, err
	}
	dataChan := n.getAnalysisBroadcaster(event.eventData.Event.GetKey()).Subscribe()
	quitChan := make(chan struct{})
	eventKey := event.eventData.Event.GetKey()

	go func() {
		n.l.Debug("analysis waiting on quitChan", log.String("eventKey", eventKey))
		<-quitChan
		n.l.Debug("analysis quitChan was closed", log.String("eventKey", eventKey))
		// the broadcaster may be already closed if the event was unregistered
		if bs := n.getAnalysisBroadcaster(eventKey); bs != nil {
			bs.CancelSubscription(dataChan)
		}
	}()
	return dataChan, quitChan, nil
}

//nolint:whitespace,dupl // false positive
func (n *NatsProxy) SubscribeSnapshotData(sel *commonv1.EventSelector) (
	d <-chan *analysisv1.SnapshotData,
	q chan<- struct{},
	err error,
) {
	var event *eventContainer
	if event, err = n.getEvent(sel); err != nil {
		return nil, nil, err
	}
	dataChan := n.getSnapshotBroadcaster(event.eventData.Event.GetKey()).Subscribe()
	quitChan := make(chan struct{})
	eventKey := event.eventData.Event.GetKey()

	go func() {
		n.l.Debug("snapshot waiting on quitChan", log.String("eventKey", eventKey))
		<-quitChan
		n.l.Debug("snapshot quitChan was closed", log.String("eventKey", eventKey))
		// the broadcaster may be already closed if the event was unregistered
		if bs := n.getSnapshotBroadcaster(eventKey); bs != nil {
			bs.CancelSubscription(dataChan)
		}
	}()
	return dataChan, quitChan, nil
}

//nolint:lll // readablity
func (n *NatsProxy) HistorySnapshotData(sel *commonv1.EventSelector) []*analysisv1.SnapshotData {
	var event *eventContainer
	var err error
	if event, err = n.getEvent(sel); err != nil {
		return nil
	}

	histData, err := n.kv.Get(
		context.Background(),
		fmt.Sprintf("snapshots.%s", event.eventData.Event.GetKey()))
	if err != nil {
		n.l.Error("error getting snapshot history", log.ErrorField(err))
		return nil
	}
	conv := snapHist{}
	if entries, cErr := conv.FromBinary(histData.Value()); cErr == nil {
		n.l.Debug("got snapshot history", log.Int("len", len(entries)))
		return entries
	} else {
		n.l.Error("error converting snapshot history", log.ErrorField(cErr))
		return []*analysisv1.SnapshotData{}
	}
}

// we have one broadcaster per event which subscribes to the nats subject.
// we distribute it within this instance via our own broadcast server
//
//nolint:lll,dupl // false positive
func (n *NatsProxy) getAnalysisBroadcaster(eventKey string) broadcast.BroadcastServer[*analysisv1.Analysis] {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if ec, ok := n.events[eventKey]; ok {
		if ec.analysisBroadcaster == nil {
			l := n.l.Named("bcst.analsyis")
			sample := &analysisv1.Analysis{}
			ec.analysisBroadcaster = createEventBroadcaster(
				"analysis", eventKey, l, n, sample,
				func(arg protoreflect.ProtoMessage) *analysisv1.Analysis {
					return proto.Clone(arg.(*analysisv1.Analysis)).(*analysisv1.Analysis)
				},
			)
		}
		return ec.analysisBroadcaster.bs
	}
	return nil
}

// we have one broadcaster per event which subscribes to the nats subject.
// we distribute it within this instance via our own broadcast server
//
//nolint:lll,dupl // false positive
func (n *NatsProxy) getRacestateBroadcaster(eventKey string) broadcast.BroadcastServer[*racestatev1.PublishStateRequest] {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if ec, ok := n.events[eventKey]; ok {
		if ec.racestateBroadcaster == nil {
			l := n.l.Named("bcst.racestate")
			sample := &racestatev1.PublishStateRequest{}
			ec.racestateBroadcaster = createEventBroadcaster(
				"racestate", eventKey, l, n, sample,
				func(arg protoreflect.ProtoMessage) *racestatev1.PublishStateRequest {
					return proto.Clone(arg.(*racestatev1.PublishStateRequest)).(*racestatev1.PublishStateRequest)
				},
			)
		}
		return ec.racestateBroadcaster.bs
	}
	return nil
}

// we have one broadcaster per event which subscribes to the nats subject.
// we distribute it within this instance via our own broadcast server
//
//nolint:lll,dupl // false positive
func (n *NatsProxy) getSpeedmapBroadcaster(eventKey string) broadcast.BroadcastServer[*racestatev1.PublishSpeedmapRequest] {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if ec, ok := n.events[eventKey]; ok {
		if ec.speedmapBroadcaster == nil {
			l := n.l.Named("bcst.speedmap")
			sample := &racestatev1.PublishSpeedmapRequest{}
			ec.speedmapBroadcaster = createEventBroadcaster(
				"speedmap", eventKey, l, n, sample,
				func(arg protoreflect.ProtoMessage) *racestatev1.PublishSpeedmapRequest {
					return proto.Clone(arg.(*racestatev1.PublishSpeedmapRequest)).(*racestatev1.PublishSpeedmapRequest)
				},
			)
		}
		return ec.speedmapBroadcaster.bs
	}
	return nil
}

// we have one broadcaster per event which subscribes to the nats subject.
// we distribute it within this instance via our own broadcast server
//
//nolint:lll,dupl // false positive
func (n *NatsProxy) getSnapshotBroadcaster(eventKey string) broadcast.BroadcastServer[*analysisv1.SnapshotData] {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if ec, ok := n.events[eventKey]; ok {
		if ec.snapshotBroadcaster == nil {
			l := n.l.Named("bcst.snapshot")
			sample := &analysisv1.SnapshotData{}
			ec.snapshotBroadcaster = createEventBroadcaster(
				"snapshot", eventKey, l, n, sample,
				func(arg protoreflect.ProtoMessage) *analysisv1.SnapshotData {
					return proto.Clone(arg.(*analysisv1.SnapshotData)).(*analysisv1.SnapshotData)
				},
			)
		}
		return ec.snapshotBroadcaster.bs
	}
	return nil
}

// create a generic event broadcaster for message Type T
//
//nolint:whitespace // false positive
func createEventBroadcaster[T any](
	name, eventKey string,
	l *log.Logger,
	n *NatsProxy,
	sample protoreflect.ProtoMessage,
	createClone func(arg protoreflect.ProtoMessage) *T,
) *broadcastData[*T] {
	dataChan := make(chan *T)
	quitChan := make(chan struct{})
	bs := broadcast.NewBroadcastServer(eventKey, fmt.Sprintf("nats.%s", name), dataChan)
	var err error
	var sub *nats.Subscription
	subj := fmt.Sprintf("%s.%s", name, eventKey)
	if sub, err = n.conn.Subscribe(subj, func(msg *nats.Msg) {
		if uErr := proto.Unmarshal(msg.Data, sample); uErr != nil {
			l.Error("error unmarshalling data",
				log.String("name", name),
				log.ErrorField(uErr))
			return
		}
		x := createClone(sample)
		l.Debug("received data", log.String("name", name), log.String("eventKey", eventKey))
		dataChan <- x
	}); err != nil {
		l.Error("error subscribing to data", log.String("name", name), log.ErrorField(err))
		return nil
	}
	go func() {
		l.Debug("waiting on quitChan",
			log.String("name", name), log.String("eventKey", eventKey))
		<-quitChan
		l.Debug("quitChan was closed",
			log.String("name", name), log.String("eventKey", eventKey))
		bs.Close()
		n.unsubscribe(sub)
	}()
	return &broadcastData[*T]{
		bs:       bs,
		quitChan: quitChan,
		name:     name,
	}
}

func (n *NatsProxy) unsubscribe(sub *nats.Subscription) {
	if sub != nil && sub.IsValid() {
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

func (n *NatsProxy) stopWatch(key string, w jetstream.KeyWatcher) {
	if w != nil {
		if err := w.Stop(); err != nil {
			n.l.Debug("error stopping watch",
				log.String("key", key),
				log.ErrorField(err))
		} else {
			n.l.Debug("stopped watch",
				log.String("key", key),
			)
		}
	}
}

//nolint:whitespace // false positive
func (n *NatsProxy) getEvent(selector *commonv1.EventSelector) (
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
		return nil, proxy.ErrEventNotFound
	case *commonv1.EventSelector_Key:
		if ret, ok := n.events[selector.GetKey()]; ok {
			return ret, nil
		}
	}
	return nil, proxy.ErrEventNotFound
}

func (n *NatsProxy) setupSubscriptions() error {
	var err error
	if n.subRegister, err = n.conn.Subscribe("event.registered",
		func(msg *nats.Msg) { n.handleIncomingEventRegistered(msg) },
	); err != nil {
		return err
	}
	if n.subUnregister, err = n.conn.Subscribe("event.unregistered",
		func(msg *nats.Msg) { n.handleIncomingEventUnregistered(msg) },
	); err != nil {
		return err
	}
	return nil
}

func (n *NatsProxy) handleIncomingEventRegistered(msg *nats.Msg) {
	var regData providerv1.RegisterEventResponse
	if uErr := proto.Unmarshal(msg.Data, &regData); uErr != nil {
		n.l.Error("error unmarshalling event.registered", log.ErrorField(uErr))
		return
	}
	n.l.Debug("received event registered", log.String("eventKey", regData.Event.Key))
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.events[regData.Event.Key] = &eventContainer{
		eventData: &proxy.EventData{
			Event: regData.Event,
			Track: regData.Track,
		},
	}
}

func (n *NatsProxy) handleIncomingEventUnregistered(msg *nats.Msg) {
	n.l.Debug("received event unregistered", log.String("eventKey", string(msg.Data)))
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if n.onUnregisterCB != nil {
		selector := &commonv1.EventSelector{
			Arg: &commonv1.EventSelector_Key{Key: string(msg.Data)},
		}
		n.onUnregisterCB(selector)
	}

	// cleanup local broadcasters
	//nolint:nestif //false positive
	if ec, ok := n.events[string(msg.Data)]; ok {
		if ec.analysisBroadcaster != nil {
			close(ec.analysisBroadcaster.quitChan)
		}
		if ec.racestateBroadcaster != nil {
			close(ec.racestateBroadcaster.quitChan)
		}
		if ec.speedmapBroadcaster != nil {
			close(ec.speedmapBroadcaster.quitChan)
		}
		if ec.snapshotBroadcaster != nil {
			close(ec.snapshotBroadcaster.quitChan)
		}
		n.stopWatch(ec.eventData.Event.GetKey(), ec.watchDriverData)
	}
	delete(n.events, string(msg.Data))

	delete(n.localEvents, string(msg.Data))
}

func (n *NatsProxy) setupKV() error {
	var js jetstream.JetStream
	var err error
	if js, err = jetstream.New(n.conn); err != nil {
		return err
	}
	n.kv, err = js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{
		Bucket: "iracelog",
		TTL:    time.Hour * 24,
	})
	return err
}
