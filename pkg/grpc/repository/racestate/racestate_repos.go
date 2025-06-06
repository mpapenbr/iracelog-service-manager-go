//nolint:whitespace // can't make both editor and linter happy
package racestate

import (
	"context"
	"errors"
	"time"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/util"
)

// stores the racestate info and the protobuf message in the database
// returns the id of the created rs_info entry for further use
//
//nolint:funlen // long function
func CreateRaceState(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
	racestate *racestatev1.PublishStateRequest,
) (rsInfoID int, err error) {
	row := conn.QueryRow(ctx, `
	insert into rs_info (
		event_id, record_stamp, session_time, session_num, time_of_day,
		air_temp, track_temp,track_wetness, precipitation
	) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	returning id
		`,
		eventID, racestate.Timestamp.AsTime(), racestate.Session.SessionTime,
		racestate.Session.SessionNum, racestate.Session.TimeOfDay,
		racestate.Session.AirTemp, racestate.Session.TrackTemp,
		racestate.Session.TrackWetness, racestate.Session.Precipitation,
	)
	rsInfoID = 0
	//nolint:govet,gocritic // by design
	if err = row.Scan(&rsInfoID); err != nil {
		return 0, err
	}

	binaryMessage, err := proto.Marshal(racestate)
	if err != nil {
		return 0, err
	}
	_, err = conn.Exec(ctx, `
	insert into race_state_proto (
		rs_info_id, protodata
	) values ($1,$2)
		`,
		rsInfoID, binaryMessage,
	)
	if err != nil {
		return 0, err
	}
	for _, msg := range racestate.Messages {
		binaryMessage, err := proto.Marshal(msg)
		if err != nil {
			return 0, err
		}
		_, err = conn.Exec(ctx, `
		insert into msg_state_proto (
			rs_info_id, protodata
		) values ($1,$2)
			`,
			rsInfoID, binaryMessage,
		)
		if err != nil {
			return 0, err
		}
	}
	return rsInfoID, nil
}

// this method is used if there are driver data prior to the first race state
func CreateDummyRaceStateInfo(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
	ts time.Time,
	sessionTime float32,
	sessionNum uint32,
) (rsInfoID int, err error) {
	row := conn.QueryRow(ctx, `
	insert into rs_info (
		event_id, record_stamp, session_time, session_num, time_of_day,
		air_temp, track_temp, track_wetness, precipitation
	) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	returning id
		`,
		eventID, ts, sessionTime, sessionNum, 0, 0, 0, 0, 0,
	)
	rsInfoID = 0
	//nolint:govet,gocritic // by design
	if err = row.Scan(&rsInfoID); err != nil {
		return 0, err
	}
	return rsInfoID, nil
}

func FindNearestRaceState(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
	sessionTime float32,
) (rsInfoID int, err error) {
	row := conn.QueryRow(ctx, `
	select id from rs_info where event_id=$1 and session_time <= $2
	order by session_time desc limit 1
		`,
		eventID, sessionTime,
	)
	rsInfoID = 0
	//nolint:nestif // tricky case ;)
	if err = row.Scan(&rsInfoID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			row = conn.QueryRow(ctx,
				"select id from rs_info where event_id=$1 order by session_time asc limit 1",
				eventID,
			)
			//nolint:gocritic // by design
			if err = row.Scan(&rsInfoID); err != nil {
				return 0, err
			} else {
				return rsInfoID, nil
			}
		} else {
			return 0, err
		}
	}
	return rsInfoID, nil
}

func LoadLatest(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
) (*racestatev1.PublishStateRequest, error) {
	row := conn.QueryRow(ctx, `
select rp.protodata from race_state_proto rp
join rs_info ri on ri.id=rp.rs_info_id
where ri.event_id=$1
order by ri.id desc limit 1
		`,
		eventID,
	)
	var binaryMessage []byte
	if err := row.Scan(&binaryMessage); err != nil {
		return nil, err
	}

	racestate := &racestatev1.PublishStateRequest{}
	if err := proto.Unmarshal(binaryMessage, racestate); err != nil {
		return nil, err
	}

	return racestate, nil
}

func CollectMessages(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
) ([]*racestatev1.MessageContainer, error) {
	row, err := conn.Query(ctx, `
select ri.session_time, ri.record_stamp, rp.protodata from msg_state_proto rp
join rs_info ri on ri.id=rp.rs_info_id
where ri.event_id=$1
order by ri.id asc
		`,
		eventID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []*racestatev1.MessageContainer{}, nil
		} else {
			return nil, err
		}
	}
	ret := make([]*racestatev1.MessageContainer, 0)
	defer row.Close()
	for row.Next() {
		var binaryMessage []byte
		var recordStamp time.Time
		var sessionTime float32
		msg := &racestatev1.Message{}
		if err := row.Scan(&sessionTime, &recordStamp, &binaryMessage); err != nil {
			return nil, err
		}
		if err := proto.Unmarshal(binaryMessage, msg); err != nil {
			return nil, err
		}

		ret = append(ret, &racestatev1.MessageContainer{
			SessionTime: sessionTime,
			Timestamp:   timestamppb.New(recordStamp),
			Message:     msg,
		})
	}

	return ret, nil
}

func LoadRange(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
	startTS time.Time,
	limit int,
) (*util.RangeContainer[racestatev1.PublishStateRequest], error) {
	provider := func() (row pgx.Rows, err error) {
		row, err = conn.Query(ctx, `
select p.protodata,ri.record_stamp,ri.session_time,ri.id from race_state_proto p
join rs_info ri on ri.id=p.rs_info_id
where
ri.event_id=$1
and ri.record_stamp >= $2
order by ri.id asc limit $3
		`,
			eventID, startTS, limit,
		)
		return row, err
	}
	return loadRange(provider, limit)
}

// loads a range of entities starting at sessionTime with session sessionNum
func LoadRangeBySessionTime(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
	sessionNum uint32,
	sessionTime float64,
	limit int,
) (*util.RangeContainer[racestatev1.PublishStateRequest], error) {
	provider := func() (rows pgx.Rows, err error) {
		rows, err = conn.Query(ctx, `
select p.protodata,ri.record_stamp,ri.session_time,ri.id from race_state_proto p
join rs_info ri on ri.id=p.rs_info_id
where
ri.event_id=$1
and ri.session_num = $2
and ri.session_time >= $3
order by ri.id asc limit $4
		`,
			eventID, sessionNum, sessionTime, limit,
		)
		return rows, err
	}
	return loadRange(provider, limit)
}

// loads a range of entities starting at rsInfoId
// sessionNum is ignored
func LoadRangeByID(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
	rsInfoID int,
	limit int,
) (*util.RangeContainer[racestatev1.PublishStateRequest], error) {
	provider := func() (rows pgx.Rows, err error) {
		rows, err = conn.Query(ctx, `
select p.protodata,ri.record_stamp,ri.session_time,ri.id from race_state_proto p
join rs_info ri on ri.id=p.rs_info_id
where
ri.event_id=$1
and ri.id >= $2
order by ri.id asc limit $3
		`,
			eventID, rsInfoID, limit,
		)
		return rows, err
	}
	return loadRange(provider, limit)
}

// loads a range of entities starting at rsInfoId within a session
func LoadRangeByIDWithinSession(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
	sessionNum uint32,
	rsInfoID int,
	limit int,
) (*util.RangeContainer[racestatev1.PublishStateRequest], error) {
	provider := func() (rows pgx.Rows, err error) {
		rows, err = conn.Query(ctx, `
select p.protodata,ri.record_stamp,ri.session_time,ri.id from race_state_proto p
join rs_info ri on ri.id=p.rs_info_id
where
ri.event_id=$1
and ri.session_num = $2
and ri.id >= $3
order by ri.id asc limit $4
		`,
			eventID, sessionNum, rsInfoID, limit,
		)
		return rows, err
	}
	return loadRange(provider, limit)
}

// loadRange loads a range of racestate entries from the database
// queryProvider must include the following columns in the following order:
// protodata, record_stamp, session_time, id
func loadRange(
	queryProvider func() (pgx.Rows, error),
	limit int,
) (*util.RangeContainer[racestatev1.PublishStateRequest], error) {
	row, err := queryProvider()
	if err != nil {
		return nil, err
	}

	defer row.Close()
	ret := util.RangeContainer[racestatev1.PublishStateRequest]{
		Data: make([]*racestatev1.PublishStateRequest, 0, limit),
	}
	for row.Next() {
		var binaryMessage []byte
		if err := row.Scan(&binaryMessage,
			&ret.LastTimestamp,
			&ret.LastSessionTime,
			&ret.LastRsInfoID); err != nil {
			return nil, err
		}
		racesate := &racestatev1.PublishStateRequest{}
		if err := proto.Unmarshal(binaryMessage, racesate); err != nil {
			return nil, err
		}
		ret.Data = append(ret.Data, racesate)
	}

	return &ret, nil
}

// deletes all entries for an event from the database, returns number of rows deleted.
//
//nolint:lll // readability
func DeleteByEventID(ctx context.Context, conn repository.Querier, eventID int) (int, error) {
	var cmdTag pgconn.CommandTag
	var err error

	_, err = conn.Exec(
		ctx,
		"delete from race_state_proto where rs_info_id in (select id from rs_info where event_id=$1)",
		eventID,
	)
	if err != nil {
		return 0, err
	}
	_, err = conn.Exec(
		ctx,
		"delete from msg_state_proto where rs_info_id in (select id from rs_info where event_id=$1)",
		eventID,
	)
	if err != nil {
		return 0, err
	}
	cmdTag, err = conn.Exec(ctx, "delete from rs_info where event_id=$1", eventID)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}
