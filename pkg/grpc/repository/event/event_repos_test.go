//nolint:dupl,funlen,errcheck,gocognit //ok for this test code
package event

import (
	"context"
	"log"
	"testing"

	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	trackrepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/track"
	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

var sampleTrack = &trackv1.Track{
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

var sampleEvent = &eventv1.Event{
	Id:                1,
	Name:              "testevent",
	Key:               "eventKey",
	Description:       "testdescription",
	EventTime:         &timestamppb.Timestamp{Seconds: 1000},
	RaceloggerVersion: "0.1.0",
	TeamRacing:        true,
	MultiClass:        true,
	NumCarTypes:       2,
	IrSessionId:       1,
	TrackId:           1,
	PitSpeed:          60,
	ReplayInfo: &eventv1.ReplayInfo{
		MinTimestamp: &timestamppb.Timestamp{Seconds: 1000},
	},
	Sessions:      []*eventv1.Session{{Num: 1, Name: "RACE"}},
	NumCarClasses: 3,
}

func createSampleEntry(db *pgxpool.Pool) *eventv1.Event {
	ctx := context.Background()
	err := pgx.BeginFunc(context.Background(), db, func(tx pgx.Tx) error {
		if err := trackrepos.Create(ctx, tx, sampleTrack); err != nil {
			return err
		}
		err := Create(ctx, tx, sampleEvent)
		return err
	})
	if err != nil {
		log.Fatalf("createSampleEntry: %v\n", err)
	}

	return sampleEvent
}

func TestCreate(t *testing.T) {
	pool := testdb.InitTestDb()
	type args struct {
		event *eventv1.Event
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "new entry",
			args: args{event: &eventv1.Event{
				Name:              "other",
				Key:               "otherEventKey",
				Description:       "testdescription",
				EventTime:         &timestamppb.Timestamp{Seconds: 1000},
				RaceloggerVersion: "0.1.0",
				TeamRacing:        true,
				MultiClass:        true,
				NumCarTypes:       2,
				IrSessionId:       1,
				TrackId:           1,
				PitSpeed:          60,
				Sessions:          []*eventv1.Session{{Num: 1, Name: "RACE"}},
				NumCarClasses:     3,
			}},
		},
		{
			name:    "duplicate",
			args:    args{event: sampleEvent},
			wantErr: true,
		},
	}
	createSampleEntry(pool)
	for _, tt := range tests {
		ctx := context.Background()
		t.Run(tt.name, func(t *testing.T) {
			err := Create(ctx, pool, tt.args.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create error = %v, wantErr %v",
					err, tt.wantErr)
			}
		})
	}
}

func TestLoadById(t *testing.T) {
	pool := testdb.InitTestDb()
	sample := createSampleEntry(pool)
	type args struct {
		id int
	}
	tests := []struct {
		name    string
		args    args
		want    *eventv1.Event
		wantErr bool
	}{
		{
			name: "existing entry",
			args: args{id: int(sample.Id)},
			want: sampleEvent,
		},
		{
			name:    "unknown entry",
			args:    args{id: -1},
			wantErr: true,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
				got, err := LoadById(ctx, c.Conn(), tt.args.id)
				if (err != nil) != tt.wantErr {
					t.Errorf("LoadEventById() error = %v, wantErr %v", err, tt.wantErr)
					return err
				}
				if !proto.Equal(got, tt.want) {
					t.Errorf("LoadEventById() = %v, want %v", got, tt.want)
				}
				return nil
			})
		})
	}
}

func TestLoadByKey(t *testing.T) {
	pool := testdb.InitTestDb()
	sample := createSampleEntry(pool)
	type args struct {
		key string
	}
	tests := []struct {
		name    string
		args    args
		want    *eventv1.Event
		wantErr bool
	}{
		{
			name: "existing entry",
			args: args{key: sampleEvent.Key},
			want: sample,
		},
		{
			name:    "unknown entry",
			args:    args{key: "unknown"},
			wantErr: true,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
				got, err := LoadByKey(ctx, c.Conn(), tt.args.key)
				if (err != nil) != tt.wantErr {
					t.Errorf("LoadEventByKey() error = %v, wantErr %v", err, tt.wantErr)
					return err
				}
				if !proto.Equal(got, tt.want) {
					t.Errorf("LoadEventByKey() = %v, want %v", got, tt.want)
				}
				return nil
			})
		})
	}
}

func TestDeleteById(t *testing.T) {
	db := testdb.InitTestDb()
	sample := createSampleEntry(db)

	type args struct {
		id int
	}
	tests := []struct {
		name string

		args    args
		want    int
		wantErr bool
	}{
		{
			name: "delete_existing",
			args: args{id: int(sample.Id)},
			want: 1,
		},
		{
			name: "delete_non_existing",
			args: args{id: -1}, // doesn't exist
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
				got, err := DeleteById(ctx, c.Conn(), tt.args.id)
				if (err != nil) != tt.wantErr {
					t.Errorf("DeleteById() error = %v, wantErr %v", err, tt.wantErr)
					return nil
				}
				if got != tt.want {
					t.Errorf("DeleteById() = %v, want %v", got, tt.want)
				}
				return nil
			})
		})
	}
}
