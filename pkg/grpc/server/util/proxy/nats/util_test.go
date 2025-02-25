package nats

import (
	"testing"

	containerv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/container/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"google.golang.org/protobuf/proto"
)

func Test_eventLookupTransfer_Conversion(t *testing.T) {
	sampleData := map[string]*containerv1.EventContainer{
		"sampleKey": {
			Event: &eventv1.Event{Key: "sampleKey", Name: "sampleName"},
			Track: &trackv1.Track{Id: 12, Name: "sampleTrack"},
			// by design no owner
		},
		"otherKey": {
			Event: &eventv1.Event{Key: "otherKey", Name: "otherName"},
			Track: &trackv1.Track{Id: 14, Name: "otherTrack"},
			Owner: "someOwner",
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
		if !proto.Equal(v, result[k]) {
			t.Errorf("eventLookupTransfer.FromBinary() key %v  got %v, want %v", k, result[k], v)
		}
	}
}
