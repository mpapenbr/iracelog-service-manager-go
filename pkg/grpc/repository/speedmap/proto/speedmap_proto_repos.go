//nolint:whitespace // can't make both editor and linter happy
package proto

import (
	"context"
	"fmt"
	"time"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

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

//nolint:lll,funlen // readability
func LoadSnapshots(
	ctx context.Context,
	conn repository.Querier,
	eventId int,
	intervalSecs int,
) ([]*analysisv1.SnapshotData, error) {
	rsId, startTs, err := findStartingPoint(ctx, conn, eventId)
	if err != nil {
		return nil, err
	}

	row, err := conn.Query(ctx, `
SELECT rs.record_stamp,rs.session_time, rs.time_of_day, rs.air_temp, rs.track_temp, rs.track_wetness, rs.precipitation, sp.protodata
FROM rs_info rs
  JOIN speedmap_proto sp ON sp.rs_info_id = rs.id
WHERE rs.id IN (SELECT y.firstId
                FROM (SELECT FIRST_VALUE(x.id) OVER (PARTITION BY date_bin ($2,x.cur,x.raceStart) ORDER BY date_bin ($2,x.cur,x.raceStart)) AS firstId
                             FROM (SELECT rs.id,
                                          rs.record_stamp AS cur,
                                          TO_TIMESTAMP($3) AS raceStart
                                   FROM rs_info rs
                                     JOIN speedmap_proto sp ON sp.rs_info_id = rs.id
                                   WHERE rs.event_id = $1 and rs.id >= $4
                                   ORDER BY record_stamp) x) y
                ORDER BY y.firstId)
order by rs.record_stamp

	`, eventId, fmt.Sprintf("'%d seconds'", intervalSecs), startTs.UnixMilli(), rsId)
	if err != nil {
		return nil, err
	}
	ret := make([]*analysisv1.SnapshotData, 0)
	for row.Next() {
		var binaryMessage []byte
		var sd analysisv1.SnapshotData

		var recordStamp time.Time
		if err := row.Scan(&recordStamp, &sd.SessionTime, &sd.TimeOfDay, &sd.AirTemp, &sd.TrackTemp, &sd.TrackWetness, &sd.Precipitation, &binaryMessage); err != nil {
			return nil, err
		}
		speedmap := &racestatev1.PublishSpeedmapRequest{}
		if err := proto.Unmarshal(binaryMessage, speedmap); err != nil {
			return nil, err
		}
		sd.RecordStamp = timestamppb.New(recordStamp)
		sd.CarClassLaptimes = make(map[string]float32)
		for k, car := range speedmap.Speedmap.Data {
			sd.CarClassLaptimes[k] = car.Laptime
		}
		ret = append(ret, &sd)

	}
	return ret, nil
}

func findStartingPoint(
	ctx context.Context,
	conn repository.Querier,
	eventId int,
) (int, *time.Time, error) {
	row, err := conn.Query(ctx, `
select rs.id, rs.record_stamp, sp.protodata
from rs_info rs join speedmap_proto sp on sp.rs_info_id=rs.id
where rs.event_id=$1
order by rs.record_stamp asc

	`, eventId)
	if err != nil {
		return 0, nil, err
	}
	for row.Next() {
		var binaryMessage []byte

		var recordStamp time.Time
		var id int
		if err := row.Scan(&id, &recordStamp, &binaryMessage); err != nil {
			return 0, nil, err
		}
		speedmap := &racestatev1.PublishSpeedmapRequest{}
		if err := proto.Unmarshal(binaryMessage, speedmap); err != nil {
			return 0, nil, err
		}

		found := false
		for _, car := range speedmap.Speedmap.Data {
			found = found || car.Laptime > 0
		}
		if found {
			return id, &recordStamp, nil
		}

	}
	return 0, nil, nil
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
