//nolint:whitespace,lll // readability
package proto_test

import (
	"context"
	"testing"

	carv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/car/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stephenafamo/bob"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	carProto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/car/proto"
	racestaterepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/racestate"
	base "github.com/mpapenbr/iracelog-service-manager-go/testsupport/basedata"
	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

func createSampleRsInfo(rsRepo api.RacestateRepository, eventID int) int {
	var err error
	var id int
	req := base.SamplePublishStateRequest()
	id, err = rsRepo.CreateRacestate(context.Background(), eventID, req)
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
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	event := base.CreateSampleEvent(db)
	r := carProto.NewCarProtoRepository(db)
	rsRepo := racestaterepos.NewRacestateRepository(db)
	rsInfoID := createSampleRsInfo(rsRepo, int(event.Id))
	err := r.Create(context.Background(), rsInfoID, sampleRequest())
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}
}

func TestDelete(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	event := base.CreateSampleEvent(db)
	r := carProto.NewCarProtoRepository(db)
	rsRepo := racestaterepos.NewRacestateRepository(db)
	rsInfoID := createSampleRsInfo(rsRepo, int(event.Id))
	var err error
	err = r.Create(context.Background(), rsInfoID, sampleRequest())
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
