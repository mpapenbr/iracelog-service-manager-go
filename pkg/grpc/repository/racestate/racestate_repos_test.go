//nolint:whitespace,lll // can't make both editor and linter happy
package racestate

import (
	"context"
	"testing"

	base "github.com/mpapenbr/iracelog-service-manager-go/testsupport/basedata"
	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

func TestCreate(t *testing.T) {
	pool := testdb.InitTestDB()
	event := base.CreateSampleEvent(pool)
	var err error
	var id int
	req := base.SamplePublishSateRequest()
	id, err = CreateRaceState(context.Background(), pool, int(event.Id), req)
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}
	if id == 0 {
		t.Errorf("Create() returned id = 0")
	}
}

func TestDelete(t *testing.T) {
	pool := testdb.InitTestDB()
	event := base.CreateSampleEvent(pool)
	req := base.SamplePublishSateRequest()
	var err error
	_, err = CreateRaceState(context.Background(), pool, int(event.Id), req)
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}
	num, err := DeleteByEventID(context.Background(), pool, int(event.Id))
	if err != nil {
		t.Errorf("DeleteByEventId() error = %v", err)
	}
	if num != 1 {
		t.Errorf("DeleteByEventId() = %v, want 1", num)
	}
}

//nolint:errcheck // by design
func TestFindNearestRaceState(t *testing.T) {
	pool := testdb.InitTestDB()
	event := base.CreateSampleEvent(pool)

	req := base.SamplePublishSateRequest()

	rsInfoIDLow, _ := CreateRaceState(context.Background(), pool, int(event.Id), req)

	req.Session.SessionTime = 1100
	rsInfoIDHigh, _ := CreateRaceState(context.Background(), pool, int(event.Id), req)

	type args struct {
		sessionTime float32
	}
	tests := []struct {
		name         string
		args         args
		wantRsInfoID int
		wantErr      bool
	}{
		{"below", args{sessionTime: 0}, rsInfoIDLow, false},
		{"equal", args{sessionTime: 1000}, rsInfoIDLow, false},
		{"between", args{sessionTime: 1080}, rsInfoIDLow, false},
		{"above", args{sessionTime: 1110}, rsInfoIDHigh, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRsInfoID, err := FindNearestRaceState(
				context.Background(),
				pool,
				int(event.Id),
				tt.args.sessionTime,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindNearestRaceState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotRsInfoID != tt.wantRsInfoID {
				t.Errorf("FindNearestRaceState() = %v, want %v", gotRsInfoID, tt.wantRsInfoID)
			}
		})
	}
}
