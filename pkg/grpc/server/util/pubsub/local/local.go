package local

import (
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
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

func (l *LocalPubSub) LiveEvents() []*pubsub.EventData {
	currentEvents := l.lookup.GetEvents()
	ret := make([]*pubsub.EventData, 0, len(currentEvents))
	for _, v := range currentEvents {
		ret = append(ret, &pubsub.EventData{Event: v.Event, Track: v.Track})
	}
	return ret
}

func (l *LocalPubSub) GetEvent(sel *commonv1.EventSelector) (*pubsub.EventData, error) {
	ed, err := l.lookup.GetEvent(sel)
	if err != nil {
		return nil, err
	}
	return &pubsub.EventData{Event: ed.Event, Track: ed.Track}, nil
}
