package basedata

import (
	"context"
	"log"
	"time"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	tenantv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/tenant/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"github.com/stephenafamo/bob"
	"google.golang.org/protobuf/types/known/timestamppb"

	bobRepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob"
)

func TestTime() *timestamppb.Timestamp {
	t, _ := time.Parse(time.RFC3339, "2024-04-28T11:10:12Z")
	return timestamppb.New(t)
}

func SampleTrack() *trackv1.Track {
	return &trackv1.Track{
		Id:        1,
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
		TireInfos: []*eventv1.TireInfo{
			{Index: 0, CompoundType: "Hard"},
			{Index: 1, CompoundType: "Wet"},
		},
	}
}

func SamplePublishStateRequest() *racestatev1.PublishStateRequest {
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

func CreateSampleEvent(db bob.DB) *eventv1.Event {
	ctx := context.Background()
	sampleTrack := SampleTrack()
	sampleEvent := SampleEvent()
	tx := bobRepos.NewBobTransaction(&db)
	r := bobRepos.NewRepositories(db)
	_ = tx
	err := tx.RunInTx(ctx, func(ctx context.Context) error {
		if err := r.Track().Create(ctx, sampleTrack); err != nil {
			return err
		}
		var tenantID uint32
		if tenant, err := r.Tenant().Create(ctx, &tenantv1.CreateTenantRequest{
			Name:     "testtenant",
			ApiKey:   "testapikey",
			IsActive: true,
		}); err != nil {
			return err
		} else {
			tenantID = tenant.ID
		}
		err := r.Event().Create(ctx, sampleEvent, tenantID)
		return err
	})
	if err != nil {
		log.Fatalf("createSampleEntry: %v\n", err)
	}

	return sampleEvent
}
