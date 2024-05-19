//nolint:whitespace // can't make both linter and editor happy
package state

// Note: data of this package is stored in the table wamp

import (
	"context"
	"errors"
	"reflect"

	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
)

func Create(ctx context.Context, conn repository.Querier, state *model.DbState) error {
	_, err := conn.Exec(
		ctx,
		"insert into wampdata (event_id, data) values ($1,$2)",
		state.EventID, state.Data)
	if err != nil {
		return err
	}
	return nil
}

// deletes all entries for an event with eventID
func DeleteByEventId(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
) (int, error) {
	cmdTag, err := conn.Exec(ctx,
		"delete from wampdata where event_id=$1", eventID)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func LoadByEventId(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
	startTS float64, num int,
) ([]*model.DbState, error) {
	rows, err := conn.Query(ctx, `
	select id,event_id,data from wampdata
    where event_id=$1 and (data->'timestamp')::numeric > $2
    order by (data->'timestamp')::numeric asc
    limit $3
	`, eventID, startTS, num)
	if err != nil {
		return nil, err
	}
	ret, err := pgx.CollectRows[*model.DbState](rows,
		func(row pgx.CollectableRow) (*model.DbState, error) {
			return pgx.RowToAddrOfStructByPos[model.DbState](row)
		})
	return ret, err
}

// loads num state entries for an event start at timestamp startTS.
// the first row contains full data, all other rows just containing the delta
// information relative to the previous entry
//
//nolint:funlen //ok
func LoadByEventIdWithDelta(
	ctx context.Context,
	conn repository.Querier, eventID int, startTS float64, num int) (
	*model.StateData, []*model.StateDelta, error,
) {
	ret := make([]*model.StateDelta, 0)
	rows, err := conn.Query(ctx, `
	select data from wampdata
    where event_id=$1 and (data->'timestamp')::numeric > $2
    order by (data->'timestamp')::numeric asc
    limit $3
	`, eventID, startTS, num)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	// collect all requested states
	workStates := make([]*model.StateData, 0)
	for rows.Next() {
		var item model.StateData
		if err := rows.Scan(&item); err != nil {
			return nil, nil, err
		}
		workStates = append(workStates, &item)
	}
	if len(workStates) == 0 {
		return nil, nil, nil
	}
	// first entry is the reference
	for idx := 1; idx < len(workStates); idx++ {
		ret = append(ret, &model.StateDelta{
			Type: int(model.MTStateDelta),
			Payload: model.DeltaPayload{
				Cars: computeCarChanges(
					workStates[idx-1].Payload.Cars,
					workStates[idx].Payload.Cars),
				Session: computeSessionChanges(
					workStates[idx-1].Payload.Session,
					workStates[idx].Payload.Session),
			},
			Timestamp: workStates[idx].Timestamp,
		})
	}
	//nolint:gosec //ok
	return workStates[0], ret, nil
}

// 0: rowIndex, 1: colIndex, 2: changed value (may be any type)
func computeCarChanges(ref, cur [][]interface{}) [][3]any {
	ret := make([][3]any, 0)
	for i := range cur {
		for j := range cur[i] {
			if i < len(ref) && (j < len(ref[i])) {
				a := ref[i][j]
				b := cur[i][j]
				if !reflect.DeepEqual(a, b) {
					ret = append(ret, [3]any{i, j, cur[i][j]})
				}
			} else {
				ret = append(ret, [3]any{i, j, cur[i][j]})
			}
		}
	}
	return ret
}

func computeSessionChanges(ref, cur []interface{}) [][2]any {
	ret := make([][2]any, 0)
	for i := range cur {
		if i < len(ref) {
			if ref[i] != cur[i] {
				ret = append(ret, [2]any{i, cur[i]})
			}
		} else {
			ret = append(ret, [2]any{i, cur[i]})
		}
	}
	return ret
}
