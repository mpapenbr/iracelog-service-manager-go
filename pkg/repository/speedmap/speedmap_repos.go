package speedmap

// Note: data of this package is stored in the table wamp

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
)

func Create(conn repository.Querier, state *model.DbSpeedmap) error {
	_, err := conn.Exec(
		context.Background(),
		"insert into speedmap (event_id, data) values ($1,$2)",
		state.EventID, state.Data)
	if err != nil {
		return err
	}
	return nil
}

// deletes all entries for an event with eventID
func DeleteByEventId(conn repository.Querier, eventID int) (int, error) {
	cmdTag, err := conn.Exec(context.Background(),
		"delete from speedmap where event_id=$1", eventID)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func LoadByEventId(conn repository.Querier, eventID int) ([]*model.DbSpeedmap, error) {
	rows, err := conn.Query(context.Background(),
		fmt.Sprintf("%s where event_id=$1 order by data->'timestamp' asc", selector),
		eventID)
	//nolint:staticcheck //by design
	defer rows.Close()

	if err != nil {
		return nil, err
	}
	ret := make([]*model.DbSpeedmap, 0)
	for rows.Next() {
		var item model.DbSpeedmap
		if err := scan(&item, rows); err != nil {
			return nil, err
		}
		ret = append(ret, &item)
	}
	return ret, nil
}

//nolint:lll //ok
func LoadLatestByEventId(conn repository.Querier, eventID int) (*model.DbSpeedmap, error) {
	row := conn.QueryRow(context.Background(),
		fmt.Sprintf("%s where event_id=$1 order by data->'timestamp' desc", selector),
		eventID)
	var e model.DbSpeedmap
	err := row.Scan(&e.ID, &e.EventID, &e.Data)
	return &e, err
}

//nolint:lll //ok
func LoadLatestByEventKey(conn repository.Querier, eventKey string) (*model.DbSpeedmap, error) {
	row := conn.QueryRow(context.Background(), `
	select s.id,s.event_id,s.data from speedmap s join event e on s.event_id=e.id
	where e.event_key=$1 order by s.data->'timestamp' desc`,
		eventKey)
	var e model.DbSpeedmap
	err := row.Scan(&e.ID, &e.EventID, &e.Data)
	return &e, err
}

// little helper
const selector = string(`select id,event_id, data from speedmap`)

func scan(e *model.DbSpeedmap, rows pgx.Rows) error {
	return rows.Scan(&e.ID, &e.EventID, &e.Data)
}
