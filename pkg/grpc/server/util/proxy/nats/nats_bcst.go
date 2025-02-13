package nats

import (
	"fmt"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	livedatav1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/livedata/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/broadcast"
)

type (
	broadcastData[T any] struct {
		bs       broadcast.BroadcastServer[T]
		quitChan chan struct{}
		name     string
	}
	broadcastContainer struct {
		eventKey                string
		l                       *log.Logger
		conn                    *nats.Conn
		kv                      jetstream.KeyValue
		driverDataBcstSupporter *driverDataBcstSupport
		analysisBroadcaster     *broadcastData[*analysisv1.Analysis]
		racestateBroadcaster    *broadcastData[*racestatev1.PublishStateRequest]
		speedmapBroadcaster     *broadcastData[*racestatev1.PublishSpeedmapRequest]
		snapshotBroadcaster     *broadcastData[*analysisv1.SnapshotData]
	}
)

//nolint:whitespace // editor/linter issue
func createBroadcasters(
	eventKey string,
	conn *nats.Conn,
	kv jetstream.KeyValue,
	l *log.Logger,
) *broadcastContainer {
	ret := &broadcastContainer{
		eventKey: eventKey,
		conn:     conn,
		kv:       kv,
		l:        l.Named("bcst"),
	}
	return ret
}

func (bc *broadcastContainer) close() {
	if bc.analysisBroadcaster != nil {
		close(bc.analysisBroadcaster.quitChan)
	}
	if bc.racestateBroadcaster != nil {
		close(bc.racestateBroadcaster.quitChan)
	}
	if bc.speedmapBroadcaster != nil {
		close(bc.speedmapBroadcaster.quitChan)
	}
	if bc.snapshotBroadcaster != nil {
		close(bc.snapshotBroadcaster.quitChan)
	}
	if bc.driverDataBcstSupporter != nil {
		bc.driverDataBcstSupporter.close()
	}
}

//nolint:dupl,whitespace // false positive,editor/linter issue
func (bc *broadcastContainer) createDriverDataChannels() (
	dataChan <-chan *livedatav1.LiveDriverDataResponse,
	quitChan chan struct{},
) {
	if bc.driverDataBcstSupporter == nil {
		bc.driverDataBcstSupporter = createDriverDataBcstSupporter(
			bc.eventKey,
			bc.kv,
			bc.l)
	}
	return bc.driverDataBcstSupporter.createChannels()
}

//nolint:dupl,whitespace // false positive,editor/linter issue
func (bc *broadcastContainer) createRacestateChannels() (
	dataChan <-chan *racestatev1.PublishStateRequest,
	quitChan chan struct{},
) {
	dataChan = bc.getRacestateBroadcaster().Subscribe()
	quitChan = make(chan struct{})

	go func() {
		bc.l.Debug("racestate waiting on quitChan", log.String("eventKey", bc.eventKey))
		<-quitChan
		bc.l.Debug("racestate quitChan was closed", log.String("eventKey", bc.eventKey))
		// the broadcaster may be already closed if the event was unregistered
		if bs := bc.getRacestateBroadcaster(); bs != nil {
			bs.CancelSubscription(dataChan)
		}
	}()
	return dataChan, quitChan
}

//nolint:dupl,whitespace // false positive,editor/linter issue
func (bc *broadcastContainer) createSpeedmapChannels() (
	dataChan <-chan *racestatev1.PublishSpeedmapRequest,
	quitChan chan struct{},
) {
	dataChan = bc.getSpeedmapBroadcaster().Subscribe()
	quitChan = make(chan struct{})

	go func() {
		bc.l.Debug("speedmap waiting on quitChan", log.String("eventKey", bc.eventKey))
		<-quitChan
		bc.l.Debug("speedmap quitChan was closed", log.String("eventKey", bc.eventKey))
		// the broadcaster may be already closed if the event was unregistered
		if bs := bc.getSpeedmapBroadcaster(); bs != nil {
			bs.CancelSubscription(dataChan)
		}
	}()
	return dataChan, quitChan
}

//nolint:dupl,whitespace // false positive,editor/linter issue
func (bc *broadcastContainer) createSnapshotChannels() (
	dataChan <-chan *analysisv1.SnapshotData,
	quitChan chan struct{},
) {
	dataChan = bc.getSnapshotBroadcaster().Subscribe()
	quitChan = make(chan struct{})

	go func() {
		bc.l.Debug("snapshot waiting on quitChan", log.String("eventKey", bc.eventKey))
		<-quitChan
		bc.l.Debug("snapshot quitChan was closed", log.String("eventKey", bc.eventKey))
		// the broadcaster may be already closed if the event was unregistered
		if bs := bc.getSnapshotBroadcaster(); bs != nil {
			bs.CancelSubscription(dataChan)
		}
	}()
	return dataChan, quitChan
}

//nolint:dupl,whitespace // false positive,editor/linter issue
func (bc *broadcastContainer) createAnalysisChannels() (
	dataChan <-chan *analysisv1.Analysis,
	quitChan chan struct{},
) {
	dataChan = bc.getAnalysisBroadcaster().Subscribe()
	quitChan = make(chan struct{})

	go func() {
		bc.l.Debug("analysis waiting on quitChan", log.String("eventKey", bc.eventKey))
		<-quitChan
		bc.l.Debug("analysis quitChan was closed", log.String("eventKey", bc.eventKey))
		// the broadcaster may be already closed if the event was unregistered
		if bs := bc.getAnalysisBroadcaster(); bs != nil {
			bs.CancelSubscription(dataChan)
		}
	}()
	return dataChan, quitChan
}

// we have one broadcaster per event which subscribes to the nats subject.
// we distribute it within this instance via our own broadcast server
//
//nolint:lll,dupl // false positive
func (bc *broadcastContainer) getAnalysisBroadcaster() broadcast.BroadcastServer[*analysisv1.Analysis] {
	if bc.analysisBroadcaster == nil {
		l := bc.l.Named("analsyis")
		sample := &analysisv1.Analysis{}
		bc.analysisBroadcaster = createEventBroadcaster(
			"analysis", bc.eventKey, l, bc.conn, sample,
			func(arg protoreflect.ProtoMessage) *analysisv1.Analysis {
				//nolint:errcheck // by design
				return proto.Clone(arg.(*analysisv1.Analysis)).(*analysisv1.Analysis)
			},
		)
	}
	return bc.analysisBroadcaster.bs
}

// we have one broadcaster per event which subscribes to the nats subject.
// we distribute it within this instance via our own broadcast server
//
//nolint:lll,dupl // false positive
func (bc *broadcastContainer) getRacestateBroadcaster() broadcast.BroadcastServer[*racestatev1.PublishStateRequest] {
	if bc.racestateBroadcaster == nil {
		l := bc.l.Named("racestate")
		sample := &racestatev1.PublishStateRequest{}
		bc.racestateBroadcaster = createEventBroadcaster(
			"racestate", bc.eventKey, l, bc.conn, sample,
			func(arg protoreflect.ProtoMessage) *racestatev1.PublishStateRequest {
				//nolint:errcheck // by design
				return proto.Clone(arg.(*racestatev1.PublishStateRequest)).(*racestatev1.PublishStateRequest)
			},
		)
	}
	return bc.racestateBroadcaster.bs
}

// we have one broadcaster per event which subscribes to the nats subject.
// we distribute it within this instance via our own broadcast server
//
//nolint:lll,dupl // false positive
func (bc *broadcastContainer) getSpeedmapBroadcaster() broadcast.BroadcastServer[*racestatev1.PublishSpeedmapRequest] {
	if bc.speedmapBroadcaster == nil {
		l := bc.l.Named("speedmap")
		sample := &racestatev1.PublishSpeedmapRequest{}
		bc.speedmapBroadcaster = createEventBroadcaster(
			"speedmap", bc.eventKey, l, bc.conn, sample,
			func(arg protoreflect.ProtoMessage) *racestatev1.PublishSpeedmapRequest {
				//nolint:errcheck // by design
				return proto.Clone(arg.(*racestatev1.PublishSpeedmapRequest)).(*racestatev1.PublishSpeedmapRequest)
			},
		)
	}
	return bc.speedmapBroadcaster.bs
}

// we have one broadcaster per event which subscribes to the nats subject.
// we distribute it within this instance via our own broadcast server
//
//nolint:lll,dupl // false positive
func (bc *broadcastContainer) getSnapshotBroadcaster() broadcast.BroadcastServer[*analysisv1.SnapshotData] {
	if bc.snapshotBroadcaster == nil {
		l := bc.l.Named("snapshot")
		sample := &analysisv1.SnapshotData{}
		bc.snapshotBroadcaster = createEventBroadcaster(
			"snapshot", bc.eventKey, l, bc.conn, sample,
			func(arg protoreflect.ProtoMessage) *analysisv1.SnapshotData {
				//nolint:errcheck // by design
				return proto.Clone(arg.(*analysisv1.SnapshotData)).(*analysisv1.SnapshotData)
			},
		)
	}
	return bc.snapshotBroadcaster.bs
}

// create a generic event broadcaster for message Type T
//
//nolint:whitespace,funlen // false positive
func createEventBroadcaster[T any](
	name, eventKey string,
	l *log.Logger,
	c *nats.Conn,
	sample protoreflect.ProtoMessage,
	createClone func(arg protoreflect.ProtoMessage) *T,
) *broadcastData[*T] {
	dataChan := make(chan *T)
	quitChan := make(chan struct{})
	bs := broadcast.NewBroadcastServer(eventKey, fmt.Sprintf("nats.%s", name), dataChan)
	var err error
	var sub *nats.Subscription
	subj := fmt.Sprintf("%s.%s", name, eventKey)
	if sub, err = c.Subscribe(subj, func(msg *nats.Msg) {
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
		l.Debug("quit received for nats subscr",
			log.String("name", name), log.String("eventKey", eventKey))
		bs.Close()

		if sub != nil && sub.IsValid() {
			if err := sub.Unsubscribe(); err != nil {
				l.Debug("error unsubscribing",
					log.String("sub", sub.Subject),
					log.ErrorField(err))
			} else {
				l.Debug("unsubscribed",
					log.String("sub", sub.Subject),
				)
			}
		}
	}()
	return &broadcastData[*T]{
		bs:       bs,
		quitChan: quitChan,
		name:     name,
	}
}
