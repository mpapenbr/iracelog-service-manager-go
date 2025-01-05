package helper

import (
	livedatav1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/livedata/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
)

// creates a new LiveDriverDataResponse from a PublishDriverDataRequest
//
//nolint:lll // editor probs + readability
func ComposeLiveDriverDataResponse(a *racestatev1.PublishDriverDataRequest) *livedatav1.LiveDriverDataResponse {
	return &livedatav1.LiveDriverDataResponse{
		Timestamp:      a.Timestamp,
		Entries:        a.Entries,
		Cars:           a.Cars,
		CarClasses:     a.CarClasses,
		SessionTime:    a.SessionTime,
		CurrentDrivers: a.CurrentDrivers,
	}
}
