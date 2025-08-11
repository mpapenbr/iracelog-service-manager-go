//nolint:whitespace,lll // readability
package analysis_test

import (
	"context"
	"testing"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stephenafamo/bob"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/analysis"
	base "github.com/mpapenbr/iracelog-service-manager-go/testsupport/basedata"
	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

func TestUpsert(t *testing.T) {
	pool := testdb.InitTestDB()

	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	r := analysis.NewAnalysisRepository(db)
	event := base.CreateSampleEvent(db)

	// simulate first save
	if err := r.Upsert(context.Background(), int(event.Id),
		&analysisv1.Analysis{
			RaceOrder: []string{"1", "2", "3"},
		}); err != nil {
		t.Errorf("Create() error = %v", err)
	}

	// simulate second save

	if err := r.Upsert(context.Background(), int(event.Id),
		&analysisv1.Analysis{
			RaceOrder: []string{"4", "5", "6"},
		}); err != nil {
		t.Errorf("Create() error = %v", err)
	}

	// verify raceorder contains last values

	analysisRet, err := r.LoadByEventID(context.Background(), int(event.Id))
	if err != nil {
		t.Errorf("LoadByEventId() error = %v", err)
	}
	if diff := cmp.Diff(
		analysisRet.RaceOrder, []string{"4", "5", "6"}); diff != "" {
		t.Errorf("Data on reload not correct: %s", diff)
	}
	// check the same by event key
	analysisRet, err = r.LoadByEventKey(context.Background(), event.Key)
	if err != nil {
		t.Errorf("LoadByEventKey() error = %v", err)
	}
	if diff := cmp.Diff(
		analysisRet.RaceOrder, []string{"4", "5", "6"}); diff != "" {
		t.Errorf("Data on reload not correct: %s", diff)
	}
}

func TestDelete(t *testing.T) {
	pool := testdb.InitTestDB()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	r := analysis.NewAnalysisRepository(db)
	event := base.CreateSampleEvent(db)

	var err error
	err = r.Upsert(context.Background(), int(event.Id),
		&analysisv1.Analysis{
			RaceOrder: []string{"1", "2", "3"},
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
