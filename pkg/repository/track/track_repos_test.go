//nolint:dupl,funlen,errcheck //ok for this test code
package track

import (
	"context"
	"log"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	tcpg "github.com/mpapenbr/iracelog-service-manager-go/testsupport/tcpostgres"
)

func initTestDb() *pgxpool.Pool {
	pool := tcpg.SetupTestDb()
	tcpg.ClearAllTables(pool)
	return pool
}

func createSampleEntry(db *pgxpool.Pool) *model.DbTrack {
	track := &model.DbTrack{
		ID:   1,
		Data: model.TrackInfo{},
	}
	err := pgx.BeginFunc(context.Background(), db, func(tx pgx.Tx) error {
		err := Create(tx.Conn(), track)
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
		t.Run(tt.name, func(t *testing.T) {
			err := pool.AcquireFunc(context.Background(), func(c *pgxpool.Conn) error {
				return Create(c.Conn(), tt.args.track)
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("Create error = %v, wantErr %v",
					err, tt.wantErr)
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
			pool.AcquireFunc(context.Background(), func(c *pgxpool.Conn) error {
				got, err := LoadById(c.Conn(), tt.args.id)
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
			db.AcquireFunc(context.Background(), func(c *pgxpool.Conn) error {
				got, err := DeleteById(c.Conn(), tt.args.id)
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
