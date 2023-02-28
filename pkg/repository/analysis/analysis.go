package analysis

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
)

// deletes all entries for an event with eventID
func DeleteByEventId(conn repository.Querier, eventID int) (int, error) {
	cmdTag, err := conn.Exec(context.Background(),
		"delete from analysis where event_id=$1", eventID)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func LoadByEventId(conn repository.Querier, eventID int) (*model.DbAnalysis, error) {
	row := conn.QueryRow(context.Background(),
		fmt.Sprintf("%s where event_id=$1", selector), eventID)
	var item model.DbAnalysis
	if err := scan(&item, row); err != nil {
		return nil, err
	}
	return &item, nil
}

// little helper
const selector = string(`select id,event_id,data from analysis`)

func scan(e *model.DbAnalysis, row pgx.Row) error {
	return row.Scan(&e.ID, &e.EventID, &e.Data)
}
