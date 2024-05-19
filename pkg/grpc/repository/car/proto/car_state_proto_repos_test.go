//nolint:whitespace,lll // readability
package proto

import (
	"context"
	"testing"

	carv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/car/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"
	"github.com/jackc/pgx/v5/pgxpool"

	racestaterepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/racestate"
	base "github.com/mpapenbr/iracelog-service-manager-go/testsupport/basedata"
	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

func createSampleRsInfo(pool *pgxpool.Pool, eventId int) int {
	var err error
	var id int
	req := base.SamplePublishSateRequest()
	id, err = racestaterepos.CreateRaceState(context.Background(), pool, eventId, req)
	if err != nil {
		return 0
	}
	return id
}

func sampleRequest() *racestatev1.PublishDriverDataRequest {
	return &racestatev1.PublishDriverDataRequest{
		SessionTime: 1000,
		Cars:        []*carv1.CarInfo{},
		CarClasses:  []*carv1.CarClass{},
		Entries:     []*carv1.CarEntry{},
		Timestamp:   base.TestTime(),
	}
}

func TestCreate(t *testing.T) {
	pool := testdb.InitTestDb()
	event := base.CreateSampleEvent(pool)
	rsInfoId := createSampleRsInfo(pool, int(event.Id))
	err := Create(context.Background(), pool, rsInfoId, sampleRequest())
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}
}

func TestDelete(t *testing.T) {
	pool := testdb.InitTestDb()
	event := base.CreateSampleEvent(pool)
	rsInfoId := createSampleRsInfo(pool, int(event.Id))
	var err error
	err = Create(context.Background(), pool, rsInfoId, sampleRequest())
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
