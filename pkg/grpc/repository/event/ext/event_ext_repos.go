//nolint:whitespace // can't make both editor and linter happy
package ext

import (
	"context"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository"
)

func Upsert(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
	extra *racestatev1.ExtraInfo,
) error {
	var err error
	_, err = conn.Exec(ctx, `
	insert into event_ext (
		event_id, extra_info
	) values ($1,$2)
	on conflict (event_id) do update set extra_info=$2
		`,
		eventID, extra,
	)

	return err
}

func LoadByEventID(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
) (*racestatev1.ExtraInfo, error) {
	row := conn.QueryRow(ctx, `
	select extra_info from event_ext where event_id=$1
	`, eventID)

	extra := racestatev1.ExtraInfo{}
	if err := row.Scan(&extra); err != nil {
		return nil, err
	}

	return &extra, nil
}

// deletes an entry from the database, returns number of rows deleted.
//
//nolint:lll // readability
func DeleteByEventID(ctx context.Context, conn repository.Querier, eventID int) (int, error) {
	cmdTag, err := conn.Exec(ctx,
		"delete from event_ext where event_id=$1", eventID)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}
