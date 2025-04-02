//nolint:whitespace,lll // readability
package proto

import (
	"context"
	"testing"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	"github.com/google/go-cmp/cmp"

	base "github.com/mpapenbr/iracelog-service-manager-go/testsupport/basedata"
	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

func TestUpsert(t *testing.T) {
	pool := testdb.InitTestDB()
	event := base.CreateSampleEvent(pool)

	// simulate first save
	if err := Upsert(context.Background(), pool, int(event.Id),
		&analysisv1.Analysis{
			RaceOrder: []string{"1", "2", "3"},
		}); err != nil {
		t.Errorf("Create() error = %v", err)
	}

	// simulate second save

	if err := Upsert(context.Background(), pool, int(event.Id),
		&analysisv1.Analysis{
			RaceOrder: []string{"4", "5", "6"},
		}); err != nil {
		t.Errorf("Create() error = %v", err)
	}

	// verify raceorder contains last values

	analysis, err := LoadByEventID(context.Background(), pool, int(event.Id))
	if err != nil {
		t.Errorf("LoadByEventId() error = %v", err)
	}
	if diff := cmp.Diff(
		analysis.RaceOrder, []string{"4", "5", "6"}); diff != "" {
		t.Errorf("Data on reload not correct: %s", diff)
	}
}

func TestDelete(t *testing.T) {
	pool := testdb.InitTestDB()
	event := base.CreateSampleEvent(pool)

	var err error
	err = Upsert(context.Background(), pool, int(event.Id),
		&analysisv1.Analysis{
			RaceOrder: []string{"1", "2", "3"},
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
