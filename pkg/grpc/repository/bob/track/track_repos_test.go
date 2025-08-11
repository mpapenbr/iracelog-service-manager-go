//nolint:dupl,funlen,errcheck,gocognit //ok for this test code
package track

import (
	"context"
	"log"
	"reflect"
	"testing"

	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stephenafamo/bob"
	"gotest.tools/v3/assert"

	tcpg "github.com/mpapenbr/iracelog-service-manager-go/testsupport/tcpostgres"
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

func createSampleEntry(db bob.DB) *trackv1.Track {
	ctx := context.Background()

	err := db.RunInTx(ctx, nil, func(ctx context.Context, ex bob.Executor) error {
		r := NewTrackRepository(ex)
		return r.Create(ctx, sampleTrack)
	})
	if err != nil {
		log.Fatalf("createSampleEntry: %v\n", err)
	}

	return sampleTrack
}

func TestCreate(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	r := NewTrackRepository(db)
	type args struct {
		track *trackv1.Track
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "new entry",
			args: args{track: &trackv1.Track{
				Id:        2,
				Name:      "testtrack2",
				ShortName: "tt2",
				Config:    "testconfig2", Length: 2000, PitSpeed: 70, Sectors: []*trackv1.Sector{},
			}},
		},
		{
			name:    "duplicate",
			args:    args{track: sampleTrack},
			wantErr: true,
		},
	}
	createSampleEntry(db)
	for _, tt := range tests {
		ctx := context.Background()
		t.Run(tt.name, func(t *testing.T) {
			err := r.Create(ctx, tt.args.track)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create error = %v, wantErr %v",
					err, tt.wantErr)
			}
		})
	}
}

func TestCheckNullablePit(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	r := NewTrackRepository(db)
	type args struct {
		pitInfo *trackv1.PitInfo
	}
	tests := []struct {
		name       string
		args       args
		wantPit    *trackv1.PitInfo
		numUpdated int
	}{
		{
			name:       "pit nil",
			args:       args{pitInfo: nil},
			numUpdated: 0,
			wantPit:    sampleTrack.PitInfo,
		},
		{
			name:       "pit values",
			args:       args{pitInfo: &trackv1.PitInfo{Entry: 1, Exit: 2, LaneLength: 3}},
			numUpdated: 1,
			wantPit:    &trackv1.PitInfo{Entry: 1, Exit: 2, LaneLength: 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tcpg.ClearAllTables(pool)
			createSampleEntry(db)
			ctx := context.Background()

			var count int
			var err error
			if count, err = r.UpdatePitInfo(ctx, 1, tt.args.pitInfo); err != nil {
				t.Errorf("Could not update  pitinfo on track = %v", err)
				return
			}
			assert.Equal(t, count, tt.numUpdated)
			check, err := r.LoadByID(ctx, 1)
			if err != nil {
				t.Errorf("Could not read track = %v", err)
			}
			if !reflect.DeepEqual(check.PitInfo, tt.wantPit) {
				t.Errorf("LoadEventById() = %v, want %v", check.PitInfo, tt.wantPit)
			}

			if err != nil {
				t.Errorf("Test error = %v", err)
			}
		})
	}
}

func TestLoadById(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	r := NewTrackRepository(db)
	sample := createSampleEntry(db)
	type args struct {
		id int
	}
	tests := []struct {
		name    string
		args    args
		want    *trackv1.Track
		wantErr bool
	}{
		{
			name: "existing entry",
			args: args{id: 1},
			want: sample,
		},
		{
			name:    "unknown entry",
			args:    args{id: 2},
			wantErr: true,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			got, err := r.LoadByID(ctx, tt.args.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadEventById() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadEventById() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteById(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	r := NewTrackRepository(db)
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

			got, err := r.DeleteByID(ctx, tt.args.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteById() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("DeleteById() = %v, want %v", got, tt.want)
			}
		})
	}
}
