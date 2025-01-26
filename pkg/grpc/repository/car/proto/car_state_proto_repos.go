//nolint:whitespace // can't make both editor and linter happy
package proto

import (
	"context"
	"time"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/util"
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
) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error) {
	provider := func() (pgx.Rows, error) {
		return conn.Query(ctx, `
select cp.protodata,ri.record_stamp,ri.session_time,ri.id from car_state_proto cp
join rs_info ri on ri.id=cp.rs_info_id
where
ri.event_id=$1
and ri.record_stamp > $2
order by ri.id asc limit $3
		`,
			eventId, startTS, limit,
		)
	}
	return loadRange(provider, limit)
}

func LoadRangeBySessionTime(
	ctx context.Context,
	conn repository.Querier,
	eventId int,
	sessionTime float64,
	limit int,
) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error) {
	provider := func() (pgx.Rows, error) {
		return conn.Query(ctx, `
select cp.protodata,ri.record_stamp,ri.session_time,ri.id from car_state_proto cp
join rs_info ri on ri.id=cp.rs_info_id
where
ri.event_id=$1
and ri.session_time > $2
order by ri.id asc limit $3
		`,
			eventId, sessionTime, limit,
		)
	}
	return loadRange(provider, limit)
}

func LoadRangeById(
	ctx context.Context,
	conn repository.Querier,
	eventId int,
	rsInfoId int,
	limit int,
) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error) {
	provider := func() (pgx.Rows, error) {
		return conn.Query(ctx, `
select cp.protodata,ri.record_stamp,ri.session_time,ri.id from car_state_proto cp
join rs_info ri on ri.id=cp.rs_info_id
where
ri.event_id=$1
and ri.id >= $2
order by ri.id asc limit $3
		`,
			eventId, rsInfoId, limit,
		)
	}
	return loadRange(provider, limit)
}

// loadRange loads a range of cardata entries from the database
// queryProvider must include the following columns in the following order:
// protodata, record_stamp, session_time, id
func loadRange(
	queryProvider func() (pgx.Rows, error),
	limit int,
) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error) {
	row, err := queryProvider()
	if err != nil {
		return nil, err
	}

	defer row.Close()
	ret := util.RangeContainer[racestatev1.PublishDriverDataRequest]{
		Data: make([]*racestatev1.PublishDriverDataRequest, 0, limit),
	}
	for row.Next() {
		var binaryMessage []byte
		if err := row.Scan(&binaryMessage,
			&ret.LastTimestamp,
			&ret.LastSessionTime,
			&ret.LastRsInfoId); err != nil {
			return nil, err
		}
		driverData := &racestatev1.PublishDriverDataRequest{}
		if err := proto.Unmarshal(binaryMessage, driverData); err != nil {
			return nil, err
		}
		ret.Data = append(ret.Data, driverData)
	}

	return &ret, nil
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
