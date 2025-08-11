// since we want to use convenience methods from testsupport this test has to be moved
// into its own package
// otherwise we end up in import cycles
//
//nolint:whitespace,lll // readability
package ext_test

import (
	"context"
	"testing"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stephenafamo/bob"
	"google.golang.org/protobuf/proto"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/event/ext"
	base "github.com/mpapenbr/iracelog-service-manager-go/testsupport/basedata"
	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

func TestUpsert(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	event := base.CreateSampleEvent(db)

	r := ext.NewEventExtRepository(db)

	// simulate first save
	if err := r.Upsert(context.Background(), int(event.Id),
		&racestatev1.ExtraInfo{
			PitInfo: &trackv1.PitInfo{Entry: 0.1, Exit: 0.2, LaneLength: 200},
		}); err != nil {
		t.Errorf("Create() error = %v", err)
	}

	// simulate second save

	if err := r.Upsert(context.Background(), int(event.Id),
		&racestatev1.ExtraInfo{
			PitInfo: &trackv1.PitInfo{Entry: 0.2, Exit: 0.4, LaneLength: 400},
		}); err != nil {
		t.Errorf("Create() error = %v", err)
	}

	// verify raceorder contains last values

	pitInfo, err := r.LoadByEventID(context.Background(), int(event.Id))
	if err != nil {
		t.Errorf("LoadByEventId() error = %v", err)
	}
	if !proto.Equal(
		pitInfo.PitInfo, &trackv1.PitInfo{Entry: 0.2, Exit: 0.4, LaneLength: 400}) {
		t.Errorf("Data on reload not correct: %s", pitInfo)
	}
}

func TestDelete(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	event := base.CreateSampleEvent(db)
	r := ext.NewEventExtRepository(db)
	var err error
	err = r.Upsert(context.Background(), int(event.Id),
		&racestatev1.ExtraInfo{
			PitInfo: &trackv1.PitInfo{Entry: 0.2, Exit: 0.4, LaneLength: 400},
		})
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
