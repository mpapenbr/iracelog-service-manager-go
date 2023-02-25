//nolint:dupl,funlen,errcheck //ok for this test code
package event

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
	pool := tcpg.SetupTestDb()
	tcpg.ClearAllTables(pool)
	return pool
}

func createSampleEntry(db *pgxpool.Pool) *model.DbEvent {
	data := model.EventData{
		Info:       model.EventDataInfo{},
		Manifests:  model.Manifests{},
		ReplayInfo: model.ReplayInfo{},
	}

	event := &model.DbEvent{
		Name:        "Test",
		Key:         "testKey",
		Description: "myDescr",
		Data:        data,
	}
	err := pgx.BeginFunc(context.Background(), db, func(tx pgx.Tx) error {
		_, err := Create(tx.Conn(), event)
		return err
	})
	if err != nil {
		log.Fatalf("createSampleEntry: %v\n", err)
	}

	return event
}

func TestEventRepository_CreateEvent(t *testing.T) {
	db := initTestDb()

	type args struct {
		event *model.DbEvent
	}
	tests := []struct {
		name string

		args    args
		wantErr bool
		checks  func(toCheck *model.DbEvent)
	}{
		{
			name: "simpleCreate",

			args: args{
				event: &model.DbEvent{Name: "Test", Key: "myKey"},
			},
			checks: func(toCheck *model.DbEvent) {
				assert.NotNil(t, toCheck.ID)
				assert.NotNil(t, toCheck.RecordStamp)
				assert.Greater(t, toCheck.ID, 0)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db.AcquireFunc(context.Background(), func(c *pgxpool.Conn) error {
				got, err := Create(c.Conn(), tt.args.event)
				if (err != nil) != tt.wantErr {
					t.Errorf("EventRepository.CreateEvent() error = %v, wantErr %v",
						err, tt.wantErr)
					return nil
				}
				tt.checks(got)
				return nil
			})
		})
	}
}

func TestEventRepository_LoadEventById(t *testing.T) {
	db := initTestDb()
	sample := createSampleEntry(db)

	type args struct {
		ctx context.Context
		id  int
	}
	tests := []struct {
		name    string
		args    args
		want    *model.DbEvent
		wantErr bool
	}{
		{
			name: "load_existing",
			args: args{ctx: context.Background(), id: sample.ID},
			want: sample,
		},
		{
			name:    "load_without_id",
			args:    args{ctx: context.Background()},
			wantErr: true,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db.AcquireFunc(context.Background(), func(c *pgxpool.Conn) error {
				got, err := LoadById(c.Conn(), tt.args.id)
				if (err != nil) != tt.wantErr {
					t.Errorf("EventRepository.LoadEventById() error = %v, wantErr %v", err, tt.wantErr)
					return nil
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("EventRepository.LoadEventById() = %v, want %v", got, tt.want)
				}
				return nil
			})
		})
	}
}

func TestEventRepository_LoadEventByKey(t *testing.T) {
	db := initTestDb()
	sample := createSampleEntry(db)

	type args struct {
		key string
	}
	tests := []struct {
		name    string
		args    args
		want    *model.DbEvent
		wantErr bool
	}{
		{
			name: "load_existing",
			args: args{key: sample.Key},
			want: sample,
		},
		{
			name:    "load_without_id",
			args:    args{},
			wantErr: true,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db.AcquireFunc(context.Background(), func(c *pgxpool.Conn) error {
				got, err := LoadByKey(c.Conn(), tt.args.key)
				if (err != nil) != tt.wantErr {
					t.Errorf("EventRepository.LoadEventByKey() error = %v, wantErr %v",
						err, tt.wantErr)
					return nil
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("EventRepository.LoadEventByKey() = %v, want %v", got, tt.want)
				}
				return nil
			})
		})
	}
}

func TestEventRepository_DeleteEventById(t *testing.T) {
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
					t.Errorf("EventRepository.DeleteEventById() error = %v, wantErr %v",
						err, tt.wantErr)
					return nil
				}
				if got != tt.want {
					t.Errorf("EventRepository.DeleteEventById() = %v, want %v",
						got, tt.want)
				}
				return nil
			})
		})
	}
}
