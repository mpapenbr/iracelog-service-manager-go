package repository

import (
	"context"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	tcpg "github.com/mpapenbr/iracelog-service-manager-go/testsupport/tcpostgres"
)

func TestEventRepository_ListEvents(t *testing.T) {
	pool := tcpg.SetupTestDb()
	type fields struct {
		pool *pgxpool.Pool
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []*model.DbEvent
	}{
		// TODO: Add test cases.
		{
			name:   "niltest",
			fields: fields{pool: pool},
			args:   args{ctx: context.Background()},
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &EventRepository{
				pool: tt.fields.pool,
			}
			if got := r.ListEvents(tt.args.ctx); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EventRepository.ListEvents() = %v, want %v", got, tt.want)
			}
		})
	}
}
