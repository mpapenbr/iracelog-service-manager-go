package basedata

import (
	"context"
	"log"
	"time"

	commonv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"
	trackv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/track/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventrepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/event"
	trackrepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/track"
)

func TestTime() *timestamppb.Timestamp {
	t, _ := time.Parse(time.RFC3339, "2024-04-28T11:10:12Z")
	return timestamppb.New(t)
}

func SampleTrack() *trackv1.Track {
	return &trackv1.Track{
		Id:        &trackv1.TrackId{Id: 1},
		Name:      "testtrack",
		ShortName: "tt",
		Config:    "testconfig",
		Length:    1000,
		PitSpeed:  60,
		PitInfo: &trackv1.PitInfo{
			Entry:      1,
			Exit:       2,
			LaneLength: 3,
		},

		Sectors: []*trackv1.Sector{
			{
				Num:      1,
				StartPct: 0.5,
			},
			{
				Num:      2,
				StartPct: 0.75,
			},
		},
	}
}

func SampleEvent() *eventv1.Event {
	return &eventv1.Event{
		Id:                1,
		Name:              "testevent",
		Key:               "eventKey",
		Description:       "testdescription",
		EventTime:         &timestamppb.Timestamp{Seconds: 0o0},
		RaceloggerVersion: "0.1.0",
		TeamRacing:        true,
		MultiClass:        true,
		NumCarTypes:       2,
		IrSessionId:       1,
		TrackId:           1,
		PitSpeed:          60,
		Sessions:          []*eventv1.Session{{Num: 1, Name: "RACE"}},
		NumCarClasses:     3,
	}
}

func SamplePublishSateRequest() *racestatev1.PublishStateRequest {
	return &racestatev1.PublishStateRequest{
		Session: &racestatev1.Session{
			SessionTime:   1000,
			TimeOfDay:     43346,
			AirTemp:       20,
			TrackTemp:     25,
			TrackWetness:  1,
			Precipitation: 0.5,
		},
		Cars: []*racestatev1.Car{},
		Messages: []*racestatev1.Message{
			{CarIdx: 1, Msg: "msg1"},
		},
		Timestamp: TestTime(),
		Event:     &commonv1.EventSelector{Arg: &commonv1.EventSelector_Key{Key: "eventKey"}},
	}
}

func CreateSampleEvent(db *pgxpool.Pool) *eventv1.Event {
	ctx := context.Background()
	sampleTrack := SampleTrack()
	sampleEvent := SampleEvent()
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if err := trackrepos.Create(ctx, tx, sampleTrack); err != nil {
			return err
		}
		err := eventrepos.Create(ctx, tx, sampleEvent)
		return err
	})
	if err != nil {
		log.Fatalf("createSampleEntry: %v\n", err)
	}

	return sampleEvent
}
