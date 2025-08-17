//nolint:whitespace,lll // readability
package speedmap_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/scan"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/models"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/speedmap"
	base "github.com/mpapenbr/iracelog-service-manager-go/testsupport/basedata"
	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

func TestRawSQL(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	event := base.CreateSampleEvent(db)
	r := speedmap.NewSpeedmapRepository(db)
	rsInfoID := createSampleRsInfo(db, int(event.Id))
	_ = r
	_ = rsInfoID
	type dummyType struct {
		ID int
	}
	dbg := bob.Debug(db)
	ctx := context.Background()
	q := psql.RawQuery(`SELECT id FROM speedmap_proto WHERE rs_info_id = ?`, psql.Arg(rsInfoID))
	x, err := bob.All(ctx, dbg, q, scan.StructMapper[dummyType]())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range x {
		t.Logf("Found item: %+v", item)
	}
}

// Note: this test is a show case on how to use own aliases on tables/columns
func TestCustom(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	event := base.CreateSampleEvent(db)
	rsInfoID := createSampleRsInfo(db, int(event.Id))

	_ = rsInfoID

	rs := models.RSInfos.Columns.AliasedAs("rs")
	rsWhere := models.SelectWhere.RSInfos.AliasedAs("rs")
	type myTmp struct {
		ID          int32
		RecordStamp time.Time
		Dings       time.Time
	}

	testQ := psql.Select(
		sm.Columns(
			rs.ID,
			rs.RecordStamp,
			psql.F("to_timestamp", psql.Arg(time.Now().UnixMilli()))().As("dings"),
		),
		sm.From(models.RSInfos.Name()).As("rs"),
		rsWhere.EventID.EQ(int32(event.Id)),
	)
	dbg := bob.Debug(db)
	ctx := context.Background()
	rTest, rErr := bob.All(ctx, dbg, testQ, scan.StructMapper[myTmp]())
	if rErr != nil {
		t.Fatal(rErr)
	}
	_ = rTest
}
