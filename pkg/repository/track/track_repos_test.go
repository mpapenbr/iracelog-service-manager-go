//nolint:dupl,funlen,errcheck //ok for this test code
package track

import (
	"context"
	"log"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	tcpg "github.com/mpapenbr/iracelog-service-manager-go/testsupport/tcpostgres"
)

func initTestDb() *pgxpool.Pool {
	pool := tcpg.SetupTestDbDeprecated()
	tcpg.ClearAllTablesDeprecated(pool)
	return pool
}

func createSampleEntry(db *pgxpool.Pool) *model.DbTrack {
	track := &model.DbTrack{
		ID:   1,
		Data: model.TrackInfo{},
	}
	ctx := context.Background()
	err := pgx.BeginFunc(context.Background(), db, func(tx pgx.Tx) error {
		err := Create(ctx, tx, track)
		return err
	})
	if err != nil {
		log.Fatalf("createSampleEntry: %v\n", err)
	}

	return track
}

func TestCreate(t *testing.T) {
	pool := initTestDb()
	type args struct {
		track *model.DbTrack
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "new entry",
			args: args{track: &model.DbTrack{ID: 2, Data: model.TrackInfo{}}},
		},
		{
			name:    "duplicate",
			args:    args{track: &model.DbTrack{ID: 1, Data: model.TrackInfo{}}},
			wantErr: true,
		},
	}
	createSampleEntry(pool)
	for _, tt := range tests {
		ctx := context.Background()
		t.Run(tt.name, func(t *testing.T) {
			err := Create(ctx, pool, tt.args.track)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create error = %v, wantErr %v",
					err, tt.wantErr)
			}
		})
	}
}

func TestCheckNullablePit(t *testing.T) {
	pool := initTestDb()
	type args struct {
		track *model.DbTrack
	}
	tests := []struct {
		name    string
		args    args
		wantPit *model.PitInfo
	}{
		{
			name:    "pit nil",
			args:    args{track: &model.DbTrack{ID: 1, Data: model.TrackInfo{}}},
			wantPit: nil,
		},
		{
			name: "empty pit",
			args: args{track: &model.DbTrack{ID: 2, Data: model.TrackInfo{
				Pit: &model.PitInfo{},
			}}},
			wantPit: &model.PitInfo{},
		},
		{
			name: "pit values",
			args: args{track: &model.DbTrack{ID: 3, Data: model.TrackInfo{
				Pit: &model.PitInfo{Entry: 1, Exit: 2, LaneLength: 3},
			}}},
			wantPit: &model.PitInfo{Entry: 1, Exit: 2, LaneLength: 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
				if err := Create(ctx, c, tt.args.track); err != nil {
					t.Errorf("Could not create track = %v", err)
					return err
				}
				check, err := LoadById(ctx, c, tt.args.track.ID)
				if err != nil {
					t.Errorf("Could not read track = %v", err)
				}
				assert.Equal(t, check.Data.Pit, tt.wantPit)
				return nil
			})
			if err != nil {
				t.Errorf("Test error = %v", err)
			}
		})
	}
}

func TestLoadById(t *testing.T) {
	pool := initTestDb()
	sample := createSampleEntry(pool)
	type args struct {
		id int
	}
	tests := []struct {
		name    string
		args    args
		want    *model.DbTrack
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
			pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
				got, err := LoadById(ctx, c.Conn(), tt.args.id)
				if (err != nil) != tt.wantErr {
					t.Errorf("LoadEventById() error = %v, wantErr %v", err, tt.wantErr)
					return err
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("LoadEventById() = %v, want %v", got, tt.want)
				}
				return nil
			})
		})
	}
}

func TestDeleteById(t *testing.T) {
	db := initTestDb()
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
			args: args{id: sample.ID},
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
