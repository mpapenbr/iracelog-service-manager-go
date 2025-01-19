package nats

import (
	"testing"

	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util/proxy"
)

func Test_eventLookupTransfer_Conversion(t *testing.T) {
	sampleData := map[string]*proxy.EventData{
		"sampleKey": {
			Event: &eventv1.Event{Key: "sampleKey", Name: "sampleName"},
			Track: &trackv1.Track{Id: 12, Name: "sampleTrack"},
		},
		"otherKey": {
			Event: &eventv1.Event{Key: "otherKey", Name: "otherName"},
			Track: &trackv1.Track{Id: 14, Name: "otherTrack"},
		},
	}

	elt := eventLookupTransfer{}
	binaryData, err := elt.ToBinary(sampleData)
	if err != nil {
		t.Fatalf("eventLookupTransfer.ToBinary() error = %v", err)
	}

	result, err := elt.FromBinary(binaryData)
	if err != nil {
		t.Fatalf("eventLookupTransfer.FromBinary() error = %v", err)
	}
	for k, v := range sampleData {
		if !v.Equals(result[k]) {
			t.Errorf("eventLookupTransfer.FromBinary() key %v  got %v, want %v", k, result[k], v)
		}
	}
}
