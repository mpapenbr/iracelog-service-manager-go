//nolint:whitespace // can't make both editor and linter happy
package proto

import (
	"context"
	"time"

	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"
	"google.golang.org/protobuf/proto"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
)

func Create(
	ctx context.Context,
	conn repository.Querier,
	rsInfoId int,
	speedmap *racestatev1.PublishSpeedmapRequest,
) error {
	binaryMessage, err := proto.Marshal(speedmap)
	if err != nil {
		return err
	}
	row := conn.QueryRow(ctx, `
	insert into speedmap_proto (
		rs_info_id, protodata
	) values ($1,$2)
	returning id
		`,
		rsInfoId, binaryMessage,
	)
	id := 0
	if err := row.Scan(&id); err != nil {
		return err
	}
	return nil
}

func LoadLatest(
	ctx context.Context,
	conn repository.Querier,
	eventId int,
) (*racestatev1.PublishSpeedmapRequest, error) {
	row := conn.QueryRow(ctx, `
select p.protodata from speedmap_proto p
join rs_info ri on ri.id=p.rs_info_id
where ri.event_id=$1
order by ri.id desc limit 1
		`,
		eventId,
	)
	var binaryMessage []byte
	if err := row.Scan(&binaryMessage); err != nil {
		return nil, err
	}

	speedmap := &racestatev1.PublishSpeedmapRequest{}
	if err := proto.Unmarshal(binaryMessage, speedmap); err != nil {
		return nil, err
	}

	return speedmap, nil
}

func LoadRange(
	ctx context.Context,
	conn repository.Querier,
	eventId int,
	startTS time.Time,
	limit int,
) ([]*racestatev1.PublishSpeedmapRequest, time.Time, error) {
	row, err := conn.Query(ctx, `
select p.protodata, ri.record_stamp from speedmap_proto p
join rs_info ri on ri.id=p.rs_info_id
where
ri.event_id=$1
and ri.record_stamp > $2
order by ri.id asc limit $3
		`,
		eventId, startTS, limit,
	)
	if err != nil {
		return nil, time.Time{}, err
	}
	ret := make([]*racestatev1.PublishSpeedmapRequest, 0, limit)
	var latestRecordStamp time.Time
	for row.Next() {
		var binaryMessage []byte
		if err := row.Scan(&binaryMessage, &latestRecordStamp); err != nil {
			return nil, time.Time{}, err
		}
		speedmap := &racestatev1.PublishSpeedmapRequest{}
		if err := proto.Unmarshal(binaryMessage, speedmap); err != nil {
			return nil, time.Time{}, err
		}
		ret = append(ret, speedmap)
	}

	return ret, latestRecordStamp, nil
}

// deletes an entry from the database, returns number of rows deleted.
//
//nolint:lll // readability
func DeleteByEventId(ctx context.Context, conn repository.Querier, eventId int) (int, error) {
	cmdTag, err := conn.Exec(ctx,
		"delete from speedmap_proto where rs_info_id in (select id from rs_info where event_id=$1)", eventId)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}
