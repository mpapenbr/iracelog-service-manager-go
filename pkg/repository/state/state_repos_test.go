//nolint:funlen,lll,errcheck // ok for param tests
package state

import (
	"context"
	"log"
	"reflect"
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

func Test_computeCarChanges(t *testing.T) {
	type args struct {
		ref [][]interface{}
		cur [][]interface{}
	}
	tests := []struct {
		name string
		args args
		want [][3]any
	}{
		{name: "empty", args: args{}, want: [][3]any{}},
		{
			name: "standard",
			args: args{
				ref: [][]interface{}{
					{1, 2, 3, 8.1, "ref", []any{1.22, "ob"}, ""},
				},
				cur: [][]interface{}{
					{5, 2, 7, 8.2, "cur", 1.23, []any{"4.55", "pb"}},
				},
			},
			want: [][3]any{
				{0, 0, 5},
				{0, 2, 7},
				{0, 3, 8.2},
				{0, 4, "cur"},
				{0, 5, 1.23},
				{0, 6, []any{"4.55", "pb"}},
			},
		},
		{
			name: "no change",
			args: args{
				ref: [][]interface{}{
					{1, 2, 3},
				},
				cur: [][]interface{}{
					{1, 2, 3},
				},
			},
			want: [][3]any{},
		},
		{
			name: "interfaces",
			args: args{
				ref: [][]interface{}{
					{[]interface{}{1.22, "ob"}},
				},
				cur: [][]interface{}{
					{[]interface{}{1.22, "ob"}},
				},
			},
			want: [][3]any{},
		},
		{
			name: "additional", // only additional, omitting columns in cur not supported
			args: args{
				ref: [][]interface{}{
					{1, 2, 3},
				},
				cur: [][]interface{}{
					{1, 2, 3, 4},
				},
			},
			want: [][3]any{{0, 3, 4}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := computeCarChanges(tt.args.ref, tt.args.cur); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("computeCarChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_computeSessionChanges(t *testing.T) {
	type args struct {
		ref []interface{}
		cur []interface{}
	}
	tests := []struct {
		name string
		args args
		want [][2]any
	}{
		{name: "empty", args: args{}, want: [][2]any{}},
		{
			name: "standard",
			args: args{
				ref: []any{1, 2, "a", "b"},
				cur: []any{5, 2, "x", "y"},
			},
			want: [][2]any{{0, 5}, {2, "x"}, {3, "y"}},
		},
		{
			name: "no change",
			args: args{
				ref: []any{1, 2},
				cur: []any{1, 2},
			},
			want: [][2]any{},
		},
		{
			name: "additional", // only additional, omitting columns in cur not supported
			args: args{
				ref: []any{1, 2},
				cur: []any{1, 2, 3},
			},
			want: [][2]any{{2, 3}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := computeSessionChanges(tt.args.ref, tt.args.cur); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("computeSessionChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadByEventIdWithDelta(t *testing.T) {
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
			Create(ctx, tx.Conn(), &model.DbState{
				EventID: e.ID,
				Data: model.StateData{
					Type: 1,
					Payload: model.StatePayload{
						Cars: [][]interface{}{
							{i, 2},
						},
						Session: []any{i, 2},
					}, Timestamp: float64(i * 10.0),
				},
			})
		}
		return err
	})
	if err != nil {
		log.Fatalf("createTestdata: %v\n", err)
	}

	var type1 int = 1
	type args struct {
		startTS float64
		num     int
	}
	tests := []struct {
		name      string
		args      args
		wantRef   any
		wantDelta []int
		wantErr   bool
	}{
		{name: "all", args: args{startTS: 0, num: 10}, wantRef: &type1, wantDelta: []int{2, 2}},
		{name: "none", args: args{startTS: 0, num: 0}, wantRef: nil, wantDelta: []int{}},
		{name: "oob", args: args{startTS: 30, num: 10}, wantRef: nil, wantDelta: []int{}},
		{name: "last", args: args{startTS: 20, num: 10}, wantRef: &type1, wantDelta: []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, got1, err := LoadByEventIdWithDelta(ctx, db, dbEvent.ID, tt.args.startTS, tt.args.num)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadByEventIdWithDelta() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil {
				assert.Equal(t, tt.wantRef, &got.Type)
			} else {
				assert.Equal(t, tt.wantRef, nil)
			}
			rest := make([]int, 0)
			for _, v := range got1 {
				rest = append(rest, v.Type)
			}
			assert.Equal(t, tt.wantDelta, rest)
		})
	}
}
