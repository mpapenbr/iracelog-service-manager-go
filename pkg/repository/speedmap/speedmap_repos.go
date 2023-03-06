package speedmap

// Note: data of this package is stored in the table wamp

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
)

func Create(conn repository.Querier, state *model.DbSpeedmap) error {
	_, err := conn.Exec(
		context.Background(),
		"insert into speedmap (event_id, data) values ($1,$2)",
		state.EventID, state.Data)
	if err != nil {
		return err
	}
	return nil
}

// deletes all entries for an event with eventID
func DeleteByEventId(conn repository.Querier, eventID int) (int, error) {
	cmdTag, err := conn.Exec(context.Background(),
		"delete from speedmap where event_id=$1", eventID)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func LoadByEventId(conn repository.Querier, eventID int) ([]*model.DbSpeedmap, error) {
	rows, err := conn.Query(context.Background(),
		fmt.Sprintf("%s where event_id=$1 order by data->'timestamp' asc", selector),
		eventID)
	if err != nil {
		return nil, err
	}
	ret := make([]*model.DbSpeedmap, 0)
	for rows.Next() {
		var item model.DbSpeedmap
		if err := scan(&item, rows); err != nil {
			return nil, err
		}
		ret = append(ret, &item)
	}
	return ret, nil
}

//nolint:lll //ok
func LoadLatestByEventId(conn repository.Querier, eventID int) (*model.DbSpeedmap, error) {
	row := conn.QueryRow(context.Background(),
		fmt.Sprintf("%s where event_id=$1 order by data->'timestamp' desc", selector),
		eventID)
	var e model.DbSpeedmap
	err := row.Scan(&e.ID, &e.EventID, &e.Data)
	return &e, err
}

//nolint:lll //ok
func LoadLatestByEventKey(conn repository.Querier, eventKey string) (*model.DbSpeedmap, error) {
	row := conn.QueryRow(context.Background(), `
	select s.id,s.event_id,s.data from speedmap s join event e on s.event_id=e.id
	where e.event_key=$1 order by s.data->'timestamp' desc`,
		eventKey)
	var e model.DbSpeedmap
	err := row.Scan(&e.ID, &e.EventID, &e.Data)
	return &e, err
}

//nolint:whitespace // can't make both editor and linter happy
func LoadRange(conn repository.Querier, eventID int, tsBegin float64, num int) (
	[]*model.SpeedmapData, error,
) {
	if rows, err := conn.Query(context.Background(), `
	select data from speedmap
    where event_id=$1 and (data->'timestamp')::numeric > $2
    order by (data->'timestamp')::numeric asc
    limit $3`,
		eventID, tsBegin, num); err == nil {
		ret := make([]*model.SpeedmapData, 0)
		for rows.Next() {
			var item model.SpeedmapData
			if scanErr := rows.Scan(&item); scanErr != nil {
				return nil, scanErr
			}
			ret = append(ret, &item)
		}
		return ret, nil
	} else {
		return nil, err
	}
}

//nolint:lll,funlen // best way to keep things readable
func LoadAvgLapOverTime(conn repository.Querier, eventId, intervalSecs int) (
	[]*model.AvgLapOverTime, error,
) {
	// calculate start id and starting time for date_bin
	row := conn.QueryRow(context.Background(), `
SELECT sm.id,
       (data -> 'timestamp')::DECIMAL AS start
FROM (SELECT id,
             event_id
      FROM speedmap
      WHERE event_id = $1
      AND   data -> 'payload' -> 'data'  @? '$[*].keyvalue() ? (@.value.chunkSpeeds == 0)'
      ORDER BY id DESC LIMIT 1) s
  JOIN speedmap sm ON sm.event_id = s.event_id
WHERE sm.id > s.id
ORDER BY sm.id ASC LIMIT 1;
	`, eventId)
	var startID int
	var startTS float64
	if err := row.Scan(&startID, &startTS); errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // empty result (not enough data)
	}

	// check if there is an entry after startID where laptime >0
	row = conn.QueryRow(context.Background(), `
SELECT id,
	(data -> 'timestamp')::DECIMAL AS start
FROM speedmap
WHERE event_id = $1
AND   data -> 'payload' -> 'data' @? '$[*].keyvalue() ? (@.value.laptime > 0)'
ORDER BY id ASC LIMIT 1;
	`, eventId)
	if err := row.Scan(); errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // empty result (not enough data)
	}
	// the big one with date_bin.
	rows, err := conn.Query(context.Background(), `
SELECT s.data
FROM speedmap s
WHERE s.id IN (SELECT y.firstId
               FROM (SELECT x.id,
                            x.cur,
                            date_bin($4,x.cur,x.raceStart) AS binStart,
                            x.cur - date_bin($4,x.cur,x.raceStart) AS delta,
                            FIRST_VALUE(x.id) OVER
                            (PARTITION BY date_bin ($4,x.cur,x.raceStart)
                             ORDER BY x.cur - date_bin ($4,x.cur,x.raceStart)) AS firstId
                     FROM (SELECT sm.id,
                                  TO_TIMESTAMP((sm.data -> 'timestamp')::DECIMAL) AS cur,
                                  TO_TIMESTAMP($3) AS raceStart
                           FROM speedmap sm
                           WHERE event_id = $1
                           AND   id >= $2) x) y)
ORDER BY s.id ASC
	`, eventId, startID, startTS, fmt.Sprintf("'%d seconds'", intervalSecs))

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ret := make([]*model.AvgLapOverTime, 0)
	for rows.Next() {
		var sd model.SpeedmapData
		if err := rows.Scan(&sd); err != nil {
			return nil, err
		}
		laptimes := map[string]float64{}
		for k, v := range sd.Payload.Data {
			laptimes[k] = v.Laptime
		}
		item := model.AvgLapOverTime{
			Timestamp:   sd.Timestamp,
			SessionTime: sd.Payload.SessionTime,
			TimeOfDay:   sd.Payload.TimeOfDay,
			TrackTemp:   sd.Payload.TrackTemp,
			Laptimes:    laptimes,
		}
		ret = append(ret, &item)
	}
	return ret, nil
}

// little helper
const selector = string(`select id,event_id, data from speedmap`)

func scan(e *model.DbSpeedmap, rows pgx.Rows) error {
	return rows.Scan(&e.ID, &e.EventID, &e.Data)
}
