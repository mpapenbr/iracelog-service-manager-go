package track

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
)

func Create(conn repository.Querier, track *model.DbTrack) error {
	_, err := conn.Exec(context.Background(),
		"insert into track (id, data) values ($1,$2)",
		track.ID, track.Data)
	if err != nil {
		return err
	}
	return nil
}

// deletes an entry from the database, returns number of rows deleted.
func DeleteById(conn repository.Querier, id int) (int, error) {
	cmdTag, err := conn.Exec(context.Background(), "delete from track where id=$1", id)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func LoadById(conn repository.Querier, id int) (*model.DbTrack, error) {
	row := conn.QueryRow(context.Background(),
		fmt.Sprintf("%s where id=$1", selector), id)
	var item model.DbTrack
	if err := scan(&item, row); err != nil {
		return nil, err
	}
	return &item, nil
}

func Update(conn repository.Querier, track *model.DbTrack) (int, error) {
	cmdTag, err := conn.Exec(context.Background(),
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
