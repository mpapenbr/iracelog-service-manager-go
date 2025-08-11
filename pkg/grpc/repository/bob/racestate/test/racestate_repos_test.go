//nolint:whitespace,lll // can't make both editor and linter happy
package racestate_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stephenafamo/bob"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/racestate"
	base "github.com/mpapenbr/iracelog-service-manager-go/testsupport/basedata"
	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

func TestCreate(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	event := base.CreateSampleEvent(db)
	r := racestate.NewRacestateRepository(db)
	var err error
	var id int
	req := base.SamplePublishStateRequest()
	id, err = r.CreateRacestate(context.Background(), int(event.Id), req)
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}
	if id == 0 {
		t.Errorf("Create() returned id = 0")
	}
}

func TestDelete(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	event := base.CreateSampleEvent(db)
	r := racestate.NewRacestateRepository(db)
	req := base.SamplePublishStateRequest()
	var err error
	_, err = r.CreateRacestate(context.Background(), int(event.Id), req)
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}
	num, err := r.DeleteByEventID(context.Background(), int(event.Id))
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
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	event := base.CreateSampleEvent(db)
	r := racestate.NewRacestateRepository(db)

	req := base.SamplePublishStateRequest()

	rsInfoIDLow, _ := r.CreateRacestate(context.Background(), int(event.Id), req)

	req.Session.SessionTime = 1100
	rsInfoIDHigh, _ := r.CreateRacestate(context.Background(), int(event.Id), req)

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
			gotRsInfoID, err := r.FindNearestRacestate(
				context.Background(),
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

//nolint:errcheck // by design
func TestLoadLatest(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	event := base.CreateSampleEvent(db)
	r := racestate.NewRacestateRepository(db)

	req := base.SamplePublishStateRequest()

	r.CreateRacestate(context.Background(), int(event.Id), req)

	req.Session.SessionTime = 1100
	r.CreateRacestate(context.Background(), int(event.Id), req)

	res, err := r.LoadLatest(context.Background(), int(event.Id))
	if err != nil {
		t.Errorf("LoadLatest() error = %v", err)
	}
	if res.Session.SessionTime != 1100 {
		t.Errorf("LoadLatest() = %v, want 1100", res.Session.SessionTime)
	}
}
