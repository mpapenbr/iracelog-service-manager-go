package local

import (
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/pubsub"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

// PubSubData implementation based on local EventLookup
type LocalPubSub struct {
	pubsub.EmptyPubSubData
	lookup *utils.EventLookup
}

// NewLocalPubSub creates a new LocalPubSub
func NewLocalPubSub(lookup *utils.EventLookup) *LocalPubSub {
	return &LocalPubSub{lookup: lookup}
}

func (l LocalPubSub) PublishEventRegistered(ed *pubsub.EventData) error {
	return nil
}

func (l LocalPubSub) PublishEventUnregistered(eventKey string) error {
	return nil
}

func (e LocalPubSub) PublishRaceStateData(req *racestatev1.PublishStateRequest) error {
	return nil
}

func (l *LocalPubSub) LiveEvents() []*pubsub.EventData {
	currentEvents := l.lookup.GetEvents()
	ret := make([]*pubsub.EventData, 0, len(currentEvents))
	for _, v := range currentEvents {
		ret = append(ret, &pubsub.EventData{Event: v.Event, Track: v.Track})
	}
	return ret
}

func (l *LocalPubSub) GetEvent(sel *commonv1.EventSelector) (*pubsub.EventData, error) {
	epd, err := l.lookup.GetEvent(sel)
	if err != nil {
		return nil, err
	}
	return &pubsub.EventData{Event: epd.Event, Track: epd.Track}, nil
}

//nolint:whitespace // false positive
func (l *LocalPubSub) SubscribeRaceStateData(sel *commonv1.EventSelector) (
	<-chan *racestatev1.PublishStateRequest, error,
) {
	epd, err := l.lookup.GetEvent(sel)
	if err != nil {
		return nil, err
	}
	return epd.RacestateBroadcast.Subscribe(), nil
}

//nolint:whitespace // false positive
func (l *LocalPubSub) UnsubscribeRaceStateData(
	sel *commonv1.EventSelector,
	dataChan <-chan *racestatev1.PublishStateRequest,
) {
	epd, err := l.lookup.GetEvent(sel)
	if err != nil {
		return
	}
	epd.RacestateBroadcast.CancelSubscription(dataChan)
}
