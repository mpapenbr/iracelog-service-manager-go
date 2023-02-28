package state

// Note: data of this package is stored in the table wamp

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
)

func Create(conn *pgx.Conn, state *model.DbState) error {
	_, err := conn.Exec(
		context.Background(),
		"insert into wampdata (event_id, data) values ($1,$2)",
		state.EventID, state.Data)
	if err != nil {
		return err
	}
	return nil
}

// deletes all entries for an event with eventID
func DeleteByEventId(conn *pgx.Conn, eventID int) (int, error) {
	cmdTag, err := conn.Exec(context.Background(),
		"delete from wampdata where event_id=$1", eventID)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func LoadByEventId(conn *pgx.Conn, eventID int) ([]*model.DbState, error) {
	rows, err := conn.Query(context.Background(),
		fmt.Sprintf("%s where event_id=$1 order by data->'timestamp' asc", selector),
		eventID)
	//nolint:staticcheck //by design
	defer rows.Close()

	if err != nil {
		return nil, err
	}
	ret := make([]*model.DbState, 0)
	for rows.Next() {
		var item model.DbState
		if err := scan(&item, rows); err != nil {
			return nil, err
		}
		ret = append(ret, &item)
	}
	return ret, nil
}

// little helper
const selector = string(`select id,event_id, data from wampdata`)

func scan(e *model.DbState, rows pgx.Rows) error {
	return rows.Scan(&e.ID, &e.EventID, &e.Data)
}
