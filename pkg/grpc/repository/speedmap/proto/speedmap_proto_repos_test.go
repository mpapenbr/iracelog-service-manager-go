//nolint:whitespace,lll // readability
package proto

import (
	"context"
	"testing"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	speedmapv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/speedmap/v1"
	"github.com/jackc/pgx/v5/pgxpool"

	racestaterepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/racestate"
	base "github.com/mpapenbr/iracelog-service-manager-go/testsupport/basedata"
	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

func createSampleRsInfo(pool *pgxpool.Pool, eventID int) int {
	var err error
	var id int
	req := base.SamplePublishSateRequest()
	id, err = racestaterepos.CreateRaceState(context.Background(), pool, eventID, req)
	if err != nil {
		return 0
	}
	return id
}

func TestCreate(t *testing.T) {
	pool := testdb.InitTestDB()
	event := base.CreateSampleEvent(pool)
	rsInfoID := createSampleRsInfo(pool, int(event.Id))
	if err := Create(context.Background(), pool, rsInfoID,
		&racestatev1.PublishSpeedmapRequest{
			Speedmap:  &speedmapv1.Speedmap{},
			Timestamp: base.TestTime(),
		}); err != nil {
		t.Errorf("Create() error = %v", err)
	}
}

func TestDelete(t *testing.T) {
	pool := testdb.InitTestDB()
	event := base.CreateSampleEvent(pool)
	rsInfoID := createSampleRsInfo(pool, int(event.Id))
	var err error
	err = Create(context.Background(), pool, rsInfoID,
		&racestatev1.PublishSpeedmapRequest{
			Speedmap:  &speedmapv1.Speedmap{},
			Timestamp: base.TestTime(),
		})
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
