//nolint:whitespace,lll // readability
package speedmap_test

import (
	"context"
	"testing"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	speedmapv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/speedmap/v1"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stephenafamo/bob"

	bobRepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/speedmap"
	base "github.com/mpapenbr/iracelog-service-manager-go/testsupport/basedata"
	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

func createSampleRsInfo(db bob.DB, eventID int) int {
	var err error
	var id int
	r := bobRepos.NewRepositories(db)
	txMgr := bobRepos.NewTransactionManager(db)
	if txMgr.RunInTx(context.Background(), func(ctx context.Context) error {
		req := base.SamplePublishStateRequest()
		id, err = r.Racestate().CreateRacestate(ctx, eventID, req)
		return err
	}) != nil {
		return 0
	}
	return id
}

func TestCreate(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	event := base.CreateSampleEvent(db)
	r := speedmap.NewSpeedmapRepository(db)
	rsInfoID := createSampleRsInfo(db, int(event.Id))
	if err := r.Create(context.Background(), rsInfoID,
		&racestatev1.PublishSpeedmapRequest{
			Speedmap:  &speedmapv1.Speedmap{},
			Timestamp: base.TestTime(),
		}); err != nil {
		t.Errorf("Create() error = %v", err)
	}
}

func TestDelete(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	event := base.CreateSampleEvent(db)
	r := speedmap.NewSpeedmapRepository(db)
	rsInfoID := createSampleRsInfo(db, int(event.Id))
	var err error
	err = r.Create(context.Background(), rsInfoID,
		&racestatev1.PublishSpeedmapRequest{
			Speedmap:  &speedmapv1.Speedmap{},
			Timestamp: base.TestTime(),
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
