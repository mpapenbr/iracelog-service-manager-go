//nolint:whitespace // can't make both editor and linter happy
package event

import (
	"context"
	"fmt"
	"time"

	eventv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/event/v1"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
)

var selector = `select id, event_key, name, description, event_time,
	racelogger_version,	team_racing, multi_class, num_car_types, num_car_classes,
	ir_session_id, track_id, pit_speed,
	replay_min_timestamp, replay_min_session_time, replay_max_session_time,
	sessions from event`

func Create(ctx context.Context, conn repository.Querier, event *eventv1.Event) error {
	replayInfo := func() *eventv1.ReplayInfo {
		if event.ReplayInfo == nil {
			return &eventv1.ReplayInfo{MinTimestamp: event.EventTime}
		} else {
			return event.ReplayInfo
		}
	}
	row := conn.QueryRow(ctx, `
	insert into event (
		event_key, name, description, event_time, racelogger_version,
		team_racing, multi_class, num_car_types, num_car_classes,ir_session_id,
		track_id, pit_speed,
		replay_min_timestamp, replay_min_session_time, replay_max_session_time,
		sessions
	) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	returning id
		`,
		event.Key, event.Name, event.Description, event.EventTime.AsTime(),
		event.RaceloggerVersion, event.TeamRacing, event.MultiClass, event.NumCarTypes,
		event.NumCarClasses, event.IrSessionId, event.TrackId, event.PitSpeed,
		replayInfo().MinTimestamp.AsTime(), replayInfo().MinSessionTime,
		replayInfo().MaxSessionTime,
		event.Sessions,
	)
	if err := row.Scan(&event.Id); err != nil {
		return err
	}
	return nil
}

func LoadById(ctx context.Context, conn repository.Querier, id int) (
	*eventv1.Event, error,
) {
	row := conn.QueryRow(ctx, fmt.Sprintf("%s where id=$1", selector), id)

	return readData(row)
}

func LoadByKey(ctx context.Context, conn repository.Querier, key string) (
	*eventv1.Event, error,
) {
	row := conn.QueryRow(ctx, fmt.Sprintf("%s where event_key=$1", selector), key)
	return readData(row)
}

func LoadAll(ctx context.Context, conn repository.Querier) (
	[]*eventv1.Event, error,
) {
	row, err := conn.Query(ctx, fmt.Sprintf("%s order by event_time desc", selector))
	if err != nil {
		return nil, err
	}
	ret := make([]*eventv1.Event, 0)
	for row.Next() {
		item, err := readData(row)
		if err != nil {
			return nil, err
		}
		ret = append(ret, item)

	}
	return ret, nil
}

// deletes an entry from the database, returns number of rows deleted.
func DeleteById(ctx context.Context, conn repository.Querier, id int) (int, error) {
	cmdTag, err := conn.Exec(ctx, "delete from event where id=$1", id)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func readData(row pgx.Row) (*eventv1.Event, error) {
	var sessions []eventv1.Session
	var replayInfo eventv1.ReplayInfo
	var item eventv1.Event
	var eventTime time.Time
	var replayMinTimestamp time.Time
	if err := row.Scan(
		&item.Id, &item.Key,
		&item.Name, &item.Description, &eventTime, &item.RaceloggerVersion,
		&item.TeamRacing, &item.MultiClass, &item.NumCarTypes, &item.NumCarClasses,
		&item.IrSessionId, &item.TrackId, &item.PitSpeed,
		&replayMinTimestamp, &replayInfo.MinSessionTime, &replayInfo.MaxSessionTime,
		&sessions,
	); err != nil {
		return nil, err
	}
	item.EventTime = timestamppb.New(eventTime)
	replayInfo.MinTimestamp = timestamppb.New(replayMinTimestamp)
	item.ReplayInfo = &replayInfo

	item.Sessions = make([]*eventv1.Session, len(sessions))
	for i := range sessions {
		item.Sessions[i] = &sessions[i]
	}

	return &item, nil
}
