//nolint:whitespace //can't make both the linter and editor happy :(
package track

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
)

func Create(ctx context.Context, conn repository.Querier, track *model.DbTrack) error {
	_, err := conn.Exec(ctx,
		"insert into track (id, data) values ($1,$2)",
		track.ID, track.Data)
	if err != nil {
		return err
	}
	return nil
}

// deletes an entry from the database, returns number of rows deleted.
func DeleteById(ctx context.Context, conn repository.Querier, id int) (int, error) {
	cmdTag, err := conn.Exec(ctx, "delete from track where id=$1", id)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func LoadById(
	ctx context.Context,
	conn repository.Querier,
	id int,
) (*model.DbTrack, error) {
	row := conn.QueryRow(ctx,
		fmt.Sprintf("%s where id=$1", selector), id)
	var item model.DbTrack
	if err := scan(&item, row); err != nil {
		return nil, err
	}
	return &item, nil
}

func Update(
	ctx context.Context,
	conn repository.Querier,
	track *model.DbTrack,
) (int, error) {
	cmdTag, err := conn.Exec(ctx,
		"update track set data=$1 where id=$2", track.Data, track.ID)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

// little helper
const selector = string(`select id,data from track`)

func scan(e *model.DbTrack, row pgx.Row) error {
	return row.Scan(&e.ID, &e.Data)
}
