//nolint:dupl,funlen,errcheck //ok for this test code
package speedmap

import (
	"context"
	"log"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/event"
	tcpg "github.com/mpapenbr/iracelog-service-manager-go/testsupport/tcpostgres"
)

func initTestDb() *pgxpool.Pool {
	pool := tcpg.SetupTestDb()
	tcpg.ClearAllTables(pool)
	return pool
}

func TestLoadRange(t *testing.T) {
	db := initTestDb()
	dbEvent := &model.DbEvent{
		Name:        "Test",
		Key:         "testKey",
		Description: "myDescr",
		Data:        model.EventData{},
	}
	ctx := context.Background()
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		e, err := event.Create(ctx, tx.Conn(), dbEvent)
		for i := 1; i <= 3; i++ {
			Create(ctx, tx.Conn(), &model.DbSpeedmap{
				EventID: e.ID,
				Data:    model.SpeedmapData{Type: i, Timestamp: float64(i * 10.0)},
			})
		}
		return err
	})
	if err != nil {
		log.Fatalf("createTestdata: %v\n", err)
	}

	type args struct {
		tsBegin float64
		num     int
	}
	tests := []struct {
		name    string
		args    args
		wantRet []int
		wantErr bool
	}{
		{name: "all", args: args{tsBegin: 0, num: 9}, wantRet: []int{1, 2, 3}},
		{name: "none", args: args{tsBegin: 0, num: 0}, wantRet: []int{}},
		{name: "oob-upper", args: args{tsBegin: 40.0, num: 10}, wantRet: []int{}},
		{name: "last", args: args{tsBegin: 20.0, num: 10}, wantRet: []int{3}},
		{name: "just two", args: args{tsBegin: 0.0, num: 2}, wantRet: []int{1, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			gotRet, err := LoadRange(ctx, db, dbEvent.ID, tt.args.tsBegin, tt.args.num)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			extractTypes := make([]int, 0)
			for _, v := range gotRet {
				extractTypes = append(extractTypes, v.Type)
			}
			assert.Equal(t, tt.wantRet, extractTypes)
		})
	}
}
