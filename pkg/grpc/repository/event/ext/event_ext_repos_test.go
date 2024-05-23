//nolint:whitespace,lll // readability
package ext

import (
	"context"
	"testing"

	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"
	trackv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/track/v1"
	"google.golang.org/protobuf/proto"

	base "github.com/mpapenbr/iracelog-service-manager-go/testsupport/basedata"
	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

func TestUpsert(t *testing.T) {
	pool := testdb.InitTestDb()
	event := base.CreateSampleEvent(pool)

	// simulate first save
	if err := Upsert(context.Background(), pool, int(event.Id),
		&racestatev1.ExtraInfo{
			PitInfo: &trackv1.PitInfo{Entry: 0.1, Exit: 0.2, LaneLength: 200},
		}); err != nil {
		t.Errorf("Create() error = %v", err)
	}

	// simulate second save

	if err := Upsert(context.Background(), pool, int(event.Id),
		&racestatev1.ExtraInfo{
			PitInfo: &trackv1.PitInfo{Entry: 0.2, Exit: 0.4, LaneLength: 400},
		}); err != nil {
		t.Errorf("Create() error = %v", err)
	}

	// verify raceorder contains last values

	pitInfo, err := LoadByEventId(context.Background(), pool, int(event.Id))
	if err != nil {
		t.Errorf("LoadByEventId() error = %v", err)
	}
	if !proto.Equal(
		pitInfo.PitInfo, &trackv1.PitInfo{Entry: 0.2, Exit: 0.4, LaneLength: 400}) {
		t.Errorf("Data on reload not correct: %s", pitInfo)
	}
}

func TestDelete(t *testing.T) {
	pool := testdb.InitTestDb()
	event := base.CreateSampleEvent(pool)

	var err error
	err = Upsert(context.Background(), pool, int(event.Id),
		&racestatev1.ExtraInfo{
			PitInfo: &trackv1.PitInfo{Entry: 0.2, Exit: 0.4, LaneLength: 400},
		})
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}
	num, err := DeleteByEventId(context.Background(), pool, int(event.Id))
	if err != nil {
		t.Errorf("DeleteByEventId() error = %v", err)
	}
	if num != 1 {
		t.Errorf("DeleteByEventId() = %v, want 1", num)
	}
}
