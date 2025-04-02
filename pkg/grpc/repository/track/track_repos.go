//nolint:whitespace // can't make both editor and linter happy
package track

import (
	"context"
	"errors"
	"fmt"

	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository"
)

var selector = `select id, name, short_name, config, track_length,
	sectors, pit_speed, pit_entry, pit_exit, pit_lane_length
	from track`

func Create(ctx context.Context, conn repository.Querier, track *trackv1.Track) error {
	workPitInfo := track.PitInfo
	if track.PitInfo == nil {
		workPitInfo = &trackv1.PitInfo{}
	}

	_, err := conn.Exec(ctx, `
	insert into track (
		id, name, short_name, config, track_length, sectors,
		pit_speed, pit_entry, pit_exit, pit_lane_length
	) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`,
		track.Id, track.Name, track.ShortName, track.Config, track.Length,
		track.Sectors,
		track.PitSpeed, workPitInfo.Entry, workPitInfo.Exit, workPitInfo.LaneLength,
	)
	if err != nil {
		return err
	}
	return nil
}

func LoadByID(ctx context.Context, conn repository.Querier, id int) (
	*trackv1.Track, error,
) {
	row := conn.QueryRow(ctx, fmt.Sprintf("%s where id=$1", selector), id)

	return readData(row)
}

func LoadAll(ctx context.Context, conn repository.Querier) (
	[]*trackv1.Track, error,
) {
	row, err := conn.Query(ctx, fmt.Sprintf("%s order by id asc", selector))
	if err != nil {
		return nil, err
	}
	ret := make([]*trackv1.Track, 0)
	defer row.Close()
	for row.Next() {
		item, err := readData(row)
		if err != nil {
			return nil, err
		}
		ret = append(ret, item)

	}
	return ret, nil
}

//nolint:whitespace //can't make both the linter and editor happy :(
func EnsureTrack(
	ctx context.Context,
	conn repository.Querier,
	track *trackv1.Track,
) error {
	_, err := LoadByID(ctx, conn, int(track.Id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Create(ctx, conn, track)
	}
	return nil
}

//nolint:whitespace //can't make both the linter and editor happy :(
func UpdatePitInfo(
	ctx context.Context,
	conn repository.Querier,
	id int,
	pitInfo *trackv1.PitInfo) (
	int, error,
) {
	if pitInfo == nil {
		return 0, nil
	}
	cmdTag, err := conn.Exec(ctx, `
	update track set pit_entry=$1, pit_exit=$2, pit_lane_length=$3 where id=$4
	`,
		pitInfo.Entry, pitInfo.Exit, pitInfo.LaneLength, id)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

// deletes an entry from the database, returns number of rows deleted.
func DeleteByID(ctx context.Context, conn repository.Querier, id int) (int, error) {
	cmdTag, err := conn.Exec(ctx, "delete from track where id=$1", id)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func readData(row pgx.Row) (*trackv1.Track, error) {
	var item trackv1.Track
	var sectors []trackv1.Sector
	var pitInfo trackv1.PitInfo

	if err := row.Scan(&item.Id, &item.Name, &item.ShortName, &item.Config,
		&item.Length, &sectors, &item.PitSpeed,
		&pitInfo.Entry, &pitInfo.Exit, &pitInfo.LaneLength); err != nil {
		return nil, err
	}
	item.Sectors = make([]*trackv1.Sector, len(sectors))
	for i := range sectors {
		item.Sectors[i] = &sectors[i]
	}
	item.PitInfo = &pitInfo

	return &item, nil
}
