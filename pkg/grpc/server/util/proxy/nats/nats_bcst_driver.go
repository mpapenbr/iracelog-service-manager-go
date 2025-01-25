package nats

import (
	"context"
	"fmt"

	livedatav1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/livedata/v1"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/broadcast"
)

type (
	// driverData doesn't fit into the other broadcaster where we get updates
	// on a regular basis. DriverData udpates occur every now and then and
	// consumers can't wait until the next update is issued by the simulator.
	// Therefore we need to keep the latest driverData in memory and provide
	// it to consumers when they subscribe to the topic.
	driverDataBcstSupport struct {
		eventKey         string
		l                *log.Logger
		bcData           *broadcastData[*livedatav1.LiveDriverDataResponse]
		kv               jetstream.KeyValue
		watchDriverData  jetstream.KeyWatcher
		latestDriverData *livedatav1.LiveDriverDataResponse
	}
)

//nolint:whitespace // editor/linter issue
func createDriverDataBcstSupporter(
	eventKey string,
	kv jetstream.KeyValue,
	l *log.Logger,
) *driverDataBcstSupport {
	ret := &driverDataBcstSupport{
		eventKey: eventKey,
		l:        l.Named("driverData"),
		kv:       kv,
	}

	return ret
}

func (bc *driverDataBcstSupport) close() {
	if bc.bcData != nil {
		close(bc.bcData.quitChan)
	}
}

//nolint:whitespace // editor/linter issue
func (bc *driverDataBcstSupport) createChannels() (
	d <-chan *livedatav1.LiveDriverDataResponse,
	q chan struct{},
) {
	dataChan := make(chan *livedatav1.LiveDriverDataResponse)
	quitChan := make(chan struct{})
	var subDataChan <-chan *livedatav1.LiveDriverDataResponse
	go func() {
		if bc.latestDriverData != nil {
			bc.l.Debug("driverData sending latest", log.String("eventKey", bc.eventKey))
			dataChan <- bc.latestDriverData
		}
		subDataChan = bc.getDriverDataBroadcaster().Subscribe()
		bc.l.Debug("driverData sending subscribed data", log.String("eventKey", bc.eventKey))
		for d := range subDataChan {
			dataChan <- d
		}
		bc.l.Debug("driverData subscription finished", log.String("eventKey", bc.eventKey))
		close(dataChan)
	}()

	go func() {
		bc.l.Debug("driverData waiting on quitChan", log.String("eventKey", bc.eventKey))
		<-quitChan
		bc.l.Debug("driverData quitChan was closed", log.String("eventKey", bc.eventKey))
		// the broadcaster may be already closed if the event was unregistered
		if bs := bc.getDriverDataBroadcaster(); bs != nil {
			bs.CancelSubscription(subDataChan)
			close(dataChan)
		}
	}()
	return dataChan, quitChan
}

//nolint:whitespace,lll,funlen // editor/linter issue
func (bc *driverDataBcstSupport) getDriverDataBroadcaster() broadcast.BroadcastServer[*livedatav1.LiveDriverDataResponse] {
	if bc.bcData != nil {
		return bc.bcData.bs
	}

	l := bc.l.Named("driverData")
	subj := fmt.Sprintf("driverdata.%s", bc.eventKey)
	var err error
	bc.watchDriverData, err = bc.kv.Watch(context.Background(), subj)
	if err != nil {
		bc.l.Error("error watching driverdata", log.ErrorField(err))
		return nil
	}
	dataChan := make(chan *livedatav1.LiveDriverDataResponse)
	quitChan := make(chan struct{})
	name := "driverdata"
	bs := broadcast.NewBroadcastServer(bc.eventKey, fmt.Sprintf("nats.%s", name), dataChan)
	bc.bcData = &broadcastData[*livedatav1.LiveDriverDataResponse]{
		bs:       bs,
		quitChan: quitChan,
		name:     name,
	}
	go func() {
		l.Debug("waiting on quitChan",
			log.String("name", name), log.String("eventKey", bc.eventKey))
		<-quitChan
		l.Debug("quitChan was closed",
			log.String("name", name), log.String("eventKey", bc.eventKey))
		bs.Close()
		if err := bc.watchDriverData.Stop(); err != nil {
			bc.l.Debug("error stopping driverdata watch",
				log.String("key", bc.eventKey),
				log.ErrorField(err))
		}
	}()
	go func() {
		for kve := range bc.watchDriverData.Updates() {
			if kve == nil {
				bc.l.Debug("watchData nil")
				continue
			}
			var resp livedatav1.LiveDriverDataResponse
			bc.l.Debug("watchData",
				log.String("key", fmt.Sprintf("driverdata.%s", bc.eventKey)),
				log.Int("value-len", len(kve.Value())),
				log.String("op", kve.Operation().String()),
				log.Uint64("rev", kve.Revision()),
			)
			if uErr := proto.Unmarshal(kve.Value(), &resp); uErr != nil {
				bc.l.Error("error unmarshalling driverdata", log.ErrorField(uErr))
			}
			bc.l.Debug("received driverdata",
				log.String("eventKey", bc.eventKey))
			bc.latestDriverData = &resp
			dataChan <- &resp
		}
		bc.l.Debug("driverdata watch done",
			log.String("eventKey", bc.eventKey))
	}()
	return bc.bcData.bs
}
