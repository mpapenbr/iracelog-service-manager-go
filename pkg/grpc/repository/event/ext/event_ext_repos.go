//nolint:whitespace // can't make both editor and linter happy
package ext

import (
	"context"

	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
)

func Upsert(
	ctx context.Context,
	conn repository.Querier,
	eventId int,
	extra *racestatev1.ExtraInfo,
) error {
	var err error
	_, err = conn.Exec(ctx, `
	insert into event_ext (
		event_id, extra_info
	) values ($1,$2)
	on conflict (event_id) do update set extra_info=$2
		`,
		eventId, extra,
	)

	return err
}

func LoadByEventId(
	ctx context.Context,
	conn repository.Querier,
	eventId int,
) (*racestatev1.ExtraInfo, error) {
	row := conn.QueryRow(ctx, `
	select extra_info from event_ext where event_id=$1
	`, eventId)

	extra := racestatev1.ExtraInfo{}
	if err := row.Scan(&extra); err != nil {
		return nil, err
	}

	return &extra, nil
}

// deletes an entry from the database, returns number of rows deleted.
//
//nolint:lll // readability
func DeleteByEventId(ctx context.Context, conn repository.Querier, eventId int) (int, error) {
	cmdTag, err := conn.Exec(ctx,
		"delete from event_ext where event_id=$1", eventId)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}
