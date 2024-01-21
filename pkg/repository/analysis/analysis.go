//nolint:whitespace //can't make both the linter and editor happy :(
package analysis

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
)

func Create(ctx context.Context, conn repository.Querier, entry *model.DbAnalysis) (
	*model.DbAnalysis, error,
) {
	row := conn.QueryRow(context.Background(), `
	insert into analysis (event_id, data)
	values ($1,$2)
	returning id
	`, entry.EventID, entry.Data)

	if err := row.Scan(&entry.ID); err != nil {
		return nil, err
	}

	return entry, nil
}

// deletes all entries for an event with eventID
func DeleteByEventId(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
) (int, error) {
	cmdTag, err := conn.Exec(context.Background(),
		"delete from analysis where event_id=$1", eventID)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func LoadByEventId(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
) (*model.DbAnalysis, error) {
	row := conn.QueryRow(context.Background(),
		fmt.Sprintf("%s where event_id=$1", selector), eventID)
	var item model.DbAnalysis
	if err := scan(&item, row); err != nil {
		return nil, err
	}
	return &item, nil
}

func Update(
	ctx context.Context,
	conn repository.Querier,
	analysis *model.DbAnalysis,
) (int, error) {
	cmdTag, err := conn.Exec(context.Background(),
		"update analysis set data=$1 where id=$2", analysis.Data, analysis.ID)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

// little helper
const selector = string(`select id,event_id,data from analysis`)

func scan(e *model.DbAnalysis, row pgx.Row) error {
	return row.Scan(&e.ID, &e.EventID, &e.Data)
}
