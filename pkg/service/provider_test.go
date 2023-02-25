package service

import (
	"testing"

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

// creates a register request with mininum required attributes
func sampleRegisterEventRequest() *RegisterEventRequest {
	return &RegisterEventRequest{
		EventKey: "Test",
		TrackInfo: model.TrackInfo{
			ID:   1,
			Name: "SomeTrack",
		},
		Manifests: model.Manifests{
			Car: []string{"CarNum", "CarId"},
		},
		EventInfo: model.EventDataInfo{
			RaceloggerVersion: "0.6.0",
			IrSessionId:       12345,
		},
	}
}

func TestProviderService_RegisterEvent(t *testing.T) {
	pool := initTestDb()
	type fields struct {
		pool   *pgxpool.Pool
		active ProviderLookup
	}
	type args struct {
		req *RegisterEventRequest
	}
	tests := []struct {
		name   string
		fields fields
		args   args

		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:   "fresh instance",
			fields: fields{pool: pool, active: ProviderLookup{}},
			args:   args{req: sampleRegisterEventRequest()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ProviderService{
				pool:   tt.fields.pool,
				active: tt.fields.active,
			}
			got, err := s.RegisterEvent(tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProviderService.RegisterEvent() error = %v, wantErr %v",
					err, tt.wantErr)
				return
			}
			assert.Equal(t, got.Event.Key, tt.args.req.EventKey)
			assert.NotNil(t, got.Event.ID)

			assert.NotEmpty(t, s.active)
		})
	}
}
