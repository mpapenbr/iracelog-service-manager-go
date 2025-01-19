package proxy

import (
	"testing"

	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
)

func Test_eventData_Conversion(t *testing.T) {
	sampleData := EventData{
		Event: &eventv1.Event{Key: "sampleKey", Name: "sampleName"},
		Track: &trackv1.Track{Id: 12, Name: "sampleTrack"},
	}

	binaryData, err := sampleData.ToBinary()
	if err != nil {
		t.Fatalf("eventData.ToBinary() error = %v", err)
	}
	var check EventData
	err = check.FromBinary(binaryData)
	if err != nil {
		t.Fatalf("eventData.FromBinary() error = %v", err)
	}

	if !check.Equals(&sampleData) {
		t.Errorf("eventData = %v, want %v", check, sampleData)
	}
}
