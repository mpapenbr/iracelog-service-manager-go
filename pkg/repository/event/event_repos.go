//nolint:whitespace // can't make both editor and linter happy
package event

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
)

func Create(ctx context.Context, conn repository.Querier, event *model.DbEvent) (
	*model.DbEvent, error,
) {
	row := conn.QueryRow(ctx, `
	insert into event (event_key,name,description, data)
	values ($1,$2,$3,$4)
	returning id,record_stamp
	`, event.Key, event.Name, event.Description, event.Data)

	if err := row.Scan(&event.ID, &event.RecordStamp); err != nil {
		return nil, err
	}

	return event, nil
}

func CreateExtra(
	ctx context.Context,
	conn repository.Querier,
	event *model.DbEventExtra,
) error {
	_, err := conn.Exec(ctx, `
	insert into event_ext (event_id, data)	values ($1,$2)`,
		event.EventID, event.Data)

	return err
}

// deletes the extra event data, returns number of rows deleted.
func DeleteExtraById(
	ctx context.Context,
	conn repository.Querier,
	id int,
) (int, error) {
	cmdTag, err := conn.Exec(ctx,
		"delete from event_ext where event_id=$1", id)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

// deletes an entry from the database, returns number of rows deleted.
func DeleteById(ctx context.Context, conn repository.Querier, id int) (int, error) {
	cmdTag, err := conn.Exec(ctx, "delete from event where id=$1", id)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func LoadById(
	ctx context.Context,
	conn repository.Querier,
	id int,
) (*model.DbEvent, error) {
	row := conn.QueryRow(ctx,
		fmt.Sprintf("%s where id=$1", selector), id)
	var event model.DbEvent
	if err := scan(&event, row); err != nil {
		return nil, err
	}
	return &event, nil
}

func LoadAll(
	ctx context.Context,
	conn repository.Querier) (
	ret []*model.DbEvent, err error,
) {
	var rows pgx.Rows
	if rows, err = conn.Query(ctx,
		fmt.Sprintf("%s order by record_stamp desc ", selector)); err != nil {
		return nil, err
	}

	ret, err = pgx.CollectRows[*model.DbEvent](rows,
		func(row pgx.CollectableRow) (*model.DbEvent, error) {
			return pgx.RowToAddrOfStructByPos[model.DbEvent](row)
		})
	return ret, err
}

func LoadByKey(
	ctx context.Context,
	conn repository.Querier,
	eventKey string,
) (*model.DbEvent, error) {
	row := conn.QueryRow(ctx,
		fmt.Sprintf("%s where event_key=$1", selector), eventKey)
	var event model.DbEvent
	if err := scan(&event, row); err != nil {
		return nil, err
	}
	return &event, nil
}

func UpdateReplayInfo(
	ctx context.Context,
	conn repository.Querier,
	eventKey string,
	replayInfo model.ReplayInfo) (
	int, error,
) {
	// we need this tmp struct for the jsonb_merge function
	// it will provide a nice {"replayInfo": { ... }} structure
	type tmp struct {
		ReplayInfo model.ReplayInfo `json:"replayInfo"`
	}
	jsonStr, _ := json.Marshal(tmp{ReplayInfo: replayInfo})
	sql := fmt.Sprintf(`
	update event set data=mgm_jsonb_merge(data, '%s'::jsonb) where event_key=$1
	`, jsonStr)

	cmdTag, err := conn.Exec(ctx, sql, eventKey)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

// little helper
const selector = string(`
select id,name,event_key,coalesce(description,''),record_stamp, data from event
`)

func scan(e *model.DbEvent, row pgx.Row) error {
	return row.Scan(&e.ID, &e.Name, &e.Key, &e.Description, &e.RecordStamp, &e.Data)
}
