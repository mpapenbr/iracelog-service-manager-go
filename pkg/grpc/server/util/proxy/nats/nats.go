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

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy/helper"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
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
		globalEvents   *GlobalEvents
	}
	Option         func(*NatsProxy)
	eventContainer struct {
		eventData     *proxy.EventData
		bcstContainer *broadcastContainer
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
	if err := ret.setupGlobalEvents(); err != nil {
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
	n.globalEvents.RegisterEvent(&proxy.EventData{
		Event: epd.Event,
		Track: epd.Track,
		Owner: epd.Owner,
	})
	return n.conn.Publish("event.registered", data)
}

func (n *NatsProxy) PublishEventUnregistered(eventKey string) error {
	n.globalEvents.UnregisterEvent(eventKey)
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
		dataChan, quitChan := event.bcstContainer.createRacestateChannels()
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
		dataChan, quitChan := event.bcstContainer.createSpeedmapChannels()
		return dataChan, quitChan, nil
	}
}

//nolint:whitespace,dupl // false positive
func (n *NatsProxy) SubscribeDriverData(sel *commonv1.EventSelector) (
	d <-chan *livedatav1.LiveDriverDataResponse,
	q chan<- struct{},
	err error,
) {
	if event, err := n.getEvent(sel); err != nil {
		return nil, nil, err
	} else {
		dataChan, quitChan := event.bcstContainer.createDriverDataChannels()
		return dataChan, quitChan, nil
	}
}

//nolint:whitespace,dupl // false positive
func (n *NatsProxy) SubscribeAnalysisData(sel *commonv1.EventSelector) (
	d <-chan *analysisv1.Analysis,
	q chan<- struct{},
	err error,
) {
	if event, err := n.getEvent(sel); err != nil {
		return nil, nil, err
	} else {
		dataChan, quitChan := event.bcstContainer.createAnalysisChannels()
		return dataChan, quitChan, nil
	}
}

//nolint:whitespace,dupl // false positive
func (n *NatsProxy) SubscribeSnapshotData(sel *commonv1.EventSelector) (
	d <-chan *analysisv1.SnapshotData,
	q chan<- struct{},
	err error,
) {
	if event, err := n.getEvent(sel); err != nil {
		return nil, nil, err
	} else {
		dataChan, quitChan := event.bcstContainer.createSnapshotChannels()
		return dataChan, quitChan, nil
	}
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
		bcstContainer: createBroadcasters(regData.Event.GetKey(), n.conn, n.kv, n.l),
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
		ec.bcstContainer.close()
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

// this will load all live events from the NATS KV store and add them to the
// events maps. This is called during startup and ensures this instance can
// provide data for all live events
func (n *NatsProxy) setupGlobalEvents() (err error) {
	if n.globalEvents, err = NewGlobalEvents(n.kv, n.l.Named("gobal")); err != nil {
		return err
	}
	var curEvents map[string]*proxy.EventData
	if curEvents, err = n.globalEvents.CurrentLiveEvents(); err != nil {
		return err
	}
	for k, v := range curEvents {
		n.events[k] = &eventContainer{
			eventData:     v,
			bcstContainer: createBroadcasters(v.Event.GetKey(), n.conn, n.kv, n.l),
		}
	}
	return nil
}
