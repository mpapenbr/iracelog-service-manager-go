package car

// Note: data of this package is stored in the table wamp

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
)

func Create(conn repository.Querier, state *model.DbCar) error {
	_, err := conn.Exec(
		context.Background(),
		"insert into car (event_id, data) values ($1,$2)",
		state.EventID, state.Data)
	if err != nil {
		return err
	}
	return nil
}

// deletes all entries for an event with eventID
func DeleteByEventId(conn repository.Querier, eventID int) (int, error) {
	cmdTag, err := conn.Exec(context.Background(),
		"delete from car where event_id=$1", eventID)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func LoadByEventId(conn repository.Querier, eventID int) ([]*model.DbCar, error) {
	rows, err := conn.Query(context.Background(),
		fmt.Sprintf("%s where event_id=$1 order by data->'timestamp' asc", selector),
		eventID)
	//nolint:staticcheck //by design
	defer rows.Close()

	if err != nil {
		return nil, err
	}
	ret := make([]*model.DbCar, 0)
	for rows.Next() {
		var item model.DbCar
		if err := scan(&item, rows); err != nil {
			return nil, err
		}
		ret = append(ret, &item)
	}
	return ret, nil
}

func LoadLatestByEventId(conn repository.Querier, eventID int) (*model.DbCar, error) {
	row := conn.QueryRow(context.Background(),
		fmt.Sprintf("%s where event_id=$1 order by data->'timestamp' desc", selector),
		eventID)
	var e model.DbCar
	err := row.Scan(&e.ID, &e.EventID, &e.Data)
	return &e, err
}

//nolint:lll //ok
func LoadLatestByEventKey(conn repository.Querier, eventKey string) (*model.DbCar, error) {
	row := conn.QueryRow(context.Background(), `
	select c.id,c.event_id,c.data from car c join event e on c.event_id=e.id
	where e.event_key=$1 order by c.data->'timestamp' desc`,
		eventKey)
	var e model.DbCar
	err := row.Scan(&e.ID, &e.EventID, &e.Data)
	return &e, err
}

// little helper
const selector = string(`select id,event_id, data from car`)

func scan(e *model.DbCar, rows pgx.Rows) error {
	return rows.Scan(&e.ID, &e.EventID, &e.Data)
}
