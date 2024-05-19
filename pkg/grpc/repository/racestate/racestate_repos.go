//nolint:whitespace // can't make both editor and linter happy
package racestate

import (
	"context"
	"errors"
	"time"

	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
)

// stores the racestate info and the protobuf message in the database
// returns the id of the created rs_info entry for further use
//
//nolint:funlen // long function
func CreateRaceState(
	ctx context.Context,
	conn repository.Querier,
	eventId int,
	racestate *racestatev1.PublishStateRequest,
) (rsInfoId int, err error) {
	row := conn.QueryRow(ctx, `
	insert into rs_info (
		event_id, record_stamp, session_time, time_of_day, air_temp, track_temp,
		track_wetness, precipitation
	) values ($1,$2,$3,$4,$5,$6,$7,$8)
	returning id
		`,
		eventId, racestate.Timestamp.AsTime(), racestate.Session.SessionTime,
		racestate.Session.TimeOfDay, racestate.Session.AirTemp, racestate.Session.TrackTemp,
		racestate.Session.TrackWetness, racestate.Session.Precipitation,
	)
	rsInfoId = 0
	//nolint:govet,gocritic // by design
	if err = row.Scan(&rsInfoId); err != nil {
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
		rsInfoId, binaryMessage,
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
			rsInfoId, binaryMessage,
		)
		if err != nil {
			return 0, err
		}
	}
	return rsInfoId, nil
}

func FindNearestRaceState(
	ctx context.Context,
	conn repository.Querier,
	eventId int,
	sessionTime float32,
) (rsInfoId int, err error) {
	row := conn.QueryRow(ctx, `
	select id from rs_info where event_id=$1 and session_time <= $2
	order by session_time desc limit 1
		`,
		eventId, sessionTime,
	)
	rsInfoId = 0
	//nolint:nestif // tricky case ;)
	if err = row.Scan(&rsInfoId); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			row = conn.QueryRow(ctx,
				"select id from rs_info where event_id=$1 order by session_time asc limit 1",
				eventId,
			)
			//nolint:gocritic // by design
			if err = row.Scan(&rsInfoId); err != nil {
				return 0, err
			} else {
				return rsInfoId, nil
			}
		} else {
			return 0, err
		}
	}
	return rsInfoId, nil
}

func LoadLatest(
	ctx context.Context,
	conn repository.Querier,
	eventId int,
) (*racestatev1.PublishStateRequest, error) {
	row := conn.QueryRow(ctx, `
select rp.protodata from race_state_proto rp
join rs_info ri on ri.id=rp.rs_info_id
where ri.event_id=$1
order by ri.id desc limit 1
		`,
		eventId,
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
	eventId int,
) ([]*racestatev1.MessageContainer, error) {
	row, err := conn.Query(ctx, `
select ri.session_time, ri.record_stamp, rp.protodata from msg_state_proto rp
join rs_info ri on ri.id=rp.rs_info_id
where ri.event_id=$1
order by ri.id asc
		`,
		eventId,
	)
	if err != nil {
		return nil, err
	}
	ret := make([]*racestatev1.MessageContainer, 0)
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
	eventId int,
	startTS time.Time,
	limit int,
) ([]*racestatev1.PublishStateRequest, time.Time, error) {
	row, err := conn.Query(ctx, `
select p.protodata,ri.record_stamp from race_state_proto p
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
	ret := make([]*racestatev1.PublishStateRequest, 0, limit)
	var latestRecordStamp time.Time
	for row.Next() {
		var binaryMessage []byte
		if err := row.Scan(&binaryMessage, &latestRecordStamp); err != nil {
			return nil, time.Time{}, err
		}
		racesate := &racestatev1.PublishStateRequest{}
		if err := proto.Unmarshal(binaryMessage, racesate); err != nil {
			return nil, time.Time{}, err
		}
		ret = append(ret, racesate)
	}

	return ret, latestRecordStamp, nil
}

// deletes all entries for an event from the database, returns number of rows deleted.
//
//nolint:lll // readability
func DeleteByEventId(ctx context.Context, conn repository.Querier, eventId int) (int, error) {
	var cmdTag pgconn.CommandTag
	var err error

	_, err = conn.Exec(ctx,
		"delete from race_state_proto where rs_info_id in (select id from rs_info where event_id=$1)", eventId)
	if err != nil {
		return 0, err
	}
	_, err = conn.Exec(ctx,
		"delete from msg_state_proto where rs_info_id in (select id from rs_info where event_id=$1)", eventId)
	if err != nil {
		return 0, err
	}
	cmdTag, err = conn.Exec(ctx, "delete from rs_info where event_id=$1", eventId)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}
