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
	driverstate *racestatev1.PublishDriverDataRequest,
) error {
	binaryMessage, err := proto.Marshal(driverstate)
	if err != nil {
		return err
	}
	row := conn.QueryRow(ctx, `
	insert into car_state_proto (
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
) (*racestatev1.PublishDriverDataRequest, error) {
	row := conn.QueryRow(ctx, `
select cp.protodata from car_state_proto cp
join rs_info ri on ri.id=cp.rs_info_id
where ri.event_id=$1
order by ri.id desc limit 1
		`,
		eventId,
	)
	var binaryMessage []byte
	if err := row.Scan(&binaryMessage); err != nil {
		return nil, err
	}

	driversate := &racestatev1.PublishDriverDataRequest{}
	if err := proto.Unmarshal(binaryMessage, driversate); err != nil {
		return nil, err
	}

	return driversate, nil
}

func LoadRange(
	ctx context.Context,
	conn repository.Querier,
	eventId int,
	startTS time.Time,
	limit int,
) ([]*racestatev1.PublishDriverDataRequest, time.Time, error) {
	row, err := conn.Query(ctx, `
select cp.protodata,ri.record_stamp from car_state_proto cp
join rs_info ri on ri.id=cp.rs_info_id
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
	ret := make([]*racestatev1.PublishDriverDataRequest, 0, limit)
	var latestRecordStamp time.Time
	for row.Next() {
		var binaryMessage []byte
		if err := row.Scan(&binaryMessage, &latestRecordStamp); err != nil {
			return nil, time.Time{}, err
		}
		driversate := &racestatev1.PublishDriverDataRequest{}
		if err := proto.Unmarshal(binaryMessage, driversate); err != nil {
			return nil, time.Time{}, err
		}
		ret = append(ret, driversate)
	}

	return ret, latestRecordStamp, nil
}

// deletes an entry from the database, returns number of rows deleted.

//nolint:lll // readability
func DeleteByEventId(ctx context.Context, conn repository.Querier, eventId int) (int, error) {
	cmdTag, err := conn.Exec(ctx,
		"delete from car_state_proto where rs_info_id in (select id from rs_info where event_id=$1)", eventId)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}
