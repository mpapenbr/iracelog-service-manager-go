//nolint:funlen // ok for tests
package service

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/track"
	tcpg "github.com/mpapenbr/iracelog-service-manager-go/testsupport/tcpostgres"
)

func initTestDb() *pgxpool.Pool {
	pool := tcpg.SetupTestDb()
	tcpg.ClearAllTables(pool)
	return pool
}

type RequestBuilder func(req *RegisterEventRequest)

func WithPitData(pit model.PitInfo) RequestBuilder {
	return func(req *RegisterEventRequest) {
		req.TrackInfo.Pit = &pit
	}
}

func defaultPitInfo() *model.PitInfo {
	return &model.PitInfo{Entry: 1, Exit: 2, LaneLength: 3}
}

// creates a register request with mininum required attributes
func sampleRegisterEventRequest(opts ...RequestBuilder) *RegisterEventRequest {
	ret := &RegisterEventRequest{
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
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func TestProviderService_RegisterEvent(t *testing.T) {
	pool := initTestDb()
	type fields struct {
		pool   *pgxpool.Pool
		lookup ProviderLookup
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
			fields: fields{pool: pool, lookup: ProviderLookup{}},
			args:   args{req: sampleRegisterEventRequest()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ProviderService{
				pool:   tt.fields.pool,
				Lookup: tt.fields.lookup,
			}
			got, err := s.RegisterEvent(context.Background(), tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProviderService.RegisterEvent() error = %v, wantErr %v",
					err, tt.wantErr)
				return
			}
			assert.Equal(t, got.Event.Key, tt.args.req.EventKey)
			assert.NotNil(t, got.Event.ID)

			assert.NotEmpty(t, s.Lookup)
		})
	}
}

func TestProviderService_StoreEventExtra(t *testing.T) {
	pool := initTestDb()
	type fields struct {
		pool *pgxpool.Pool
	}
	type args struct {
		reqEvent RegisterEventRequest
		extra    *model.DbEventExtra
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		checks func(*testing.T, *model.DbTrack)
	}{
		// TODO: Add test cases.
		{
			name:   "track_pit_no__extra_pit_no",
			fields: fields{pool: pool},
			args: args{
				reqEvent: *sampleRegisterEventRequest(),
				extra:    &model.DbEventExtra{Data: model.ExtraInfo{}},
			},
			checks: func(t *testing.T, got *model.DbTrack) {
				t.Helper()
				assert.Nil(t, got.Data.Pit)
			},
		},
		{
			name:   "track_pit_yes__extra_pit_no",
			fields: fields{pool: pool},
			args: args{
				reqEvent: *sampleRegisterEventRequest(WithPitData(*defaultPitInfo())),
				extra:    &model.DbEventExtra{Data: model.ExtraInfo{}},
			},
			checks: func(t *testing.T, got *model.DbTrack) {
				t.Helper()
				assert.Equal(t, defaultPitInfo(), got.Data.Pit)
			},
		},
		{
			name:   "track_pit_no__extra_pit_yes",
			fields: fields{pool: pool},
			args: args{
				reqEvent: *sampleRegisterEventRequest(),
				extra: &model.DbEventExtra{
					Data: model.ExtraInfo{Track: model.TrackInfo{Pit: defaultPitInfo()}},
				},
			},
			checks: func(t *testing.T, got *model.DbTrack) {
				t.Helper()
				assert.Equal(t, defaultPitInfo(), got.Data.Pit)
			},
		},
		{
			name:   "track_pit_no__extra_pit_yes_zero",
			fields: fields{pool: pool},
			args: args{
				reqEvent: *sampleRegisterEventRequest(),
				extra: &model.DbEventExtra{Data: model.ExtraInfo{Track: model.TrackInfo{
					Pit: &model.PitInfo{Entry: 0, Exit: 0, LaneLength: 0},
				}}},
			},
			checks: func(t *testing.T, got *model.DbTrack) {
				t.Helper()
				assert.Nil(t, got.Data.Pit)
			},
		},
		{
			name:   "track_pit_yes__extra_pit_yes_no_override",
			fields: fields{pool: pool},
			args: args{
				reqEvent: *sampleRegisterEventRequest(WithPitData(*defaultPitInfo())),
				extra: &model.DbEventExtra{Data: model.ExtraInfo{Track: model.TrackInfo{
					Pit: &model.PitInfo{Entry: 6, Exit: 7, LaneLength: 8},
				}}},
			},
			checks: func(t *testing.T, got *model.DbTrack) {
				t.Helper()
				assert.Equal(t, defaultPitInfo(), got.Data.Pit)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tcpg.ClearTrackTable(tt.fields.pool)
			tcpg.ClearEventTable(tt.fields.pool)
			s := &ProviderService{
				pool: tt.fields.pool, Lookup: ProviderLookup{},
			}
			ctx := context.Background()
			//nolint:gosec //ignoring G601: implicit memory aliasing of items in for range loop
			if pd, err := s.RegisterEvent(ctx, &tt.args.reqEvent); err != nil {
				t.Errorf("ProviderService.StoreEventExtra() error = %v", err)
			} else {
				tt.args.extra.EventID = pd.Event.ID
			}

			if err := s.StoreEventExtra(ctx, tt.args.extra); err != nil {
				t.Errorf("ProviderService.StoreEventExtra() error = %v", err)
			}
			if err := pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
				dbTrack, err := track.LoadById(
					ctx,
					c.Conn(),
					tt.args.reqEvent.EventInfo.TrackId)
				if err != nil {
					return err
				}
				tt.checks(t, dbTrack)
				return nil
			}); err != nil {
				t.Errorf("ProviderService.StoreEventExtra() error = %v", err)
			}
		})
	}
}
