package event

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
)

func Create(conn *pgx.Conn, event *model.DbEvent) (*model.DbEvent, error) {
	row := conn.QueryRow(context.Background(), `
	insert into event (event_key,name,description, data)
	values ($1,$2,$3,$4)
	returning id,record_stamp
	`, event.Key, event.Name, event.Description, event.Data)

	if err := row.Scan(&event.ID, &event.RecordStamp); err != nil {
		return nil, err
	}

	return event, nil
}

// deletes an entry from the database, returns number of rows deleted.
func DeleteById(conn *pgx.Conn, id int) (int, error) {
	cmdTag, err := conn.Exec(context.Background(), "delete from event where id=$1", id)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func LoadById(conn *pgx.Conn, id int) (*model.DbEvent, error) {
	row := conn.QueryRow(context.Background(),
		fmt.Sprintf("%s where id=$1", selector), id)
	var event model.DbEvent
	if err := scan(&event, row); err != nil {
		return nil, err
	}
	return &event, nil
}

func LoadByKey(conn *pgx.Conn, eventKey string) (*model.DbEvent, error) {
	row := conn.QueryRow(context.Background(),
		fmt.Sprintf("%s where event_key=$1", selector), eventKey)
	var event model.DbEvent
	if err := scan(&event, row); err != nil {
		return nil, err
	}
	return &event, nil
}

// little helper
const selector = string(`
select id,name,event_key,coalesce(description,''),record_stamp, data from event
`)

func scan(e *model.DbEvent, rows pgx.Row) error {
	return rows.Scan(&e.ID, &e.Name, &e.Key, &e.Description, &e.RecordStamp, &e.Data)
}
