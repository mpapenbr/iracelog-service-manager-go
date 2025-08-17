//nolint:whitespace // can't make both editor and linter happy
package speedmap

import (
	"context"
	"fmt"
	"time"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/aarondl/opt/omit"
	"github.com/shopspring/decimal"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/fm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/wm"
	"github.com/stephenafamo/scan"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/models"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	bobCtx "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/context"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/util"
)

type (
	repo struct {
		conn bob.Executor
	}
	// used for custom queries with attributes from different tables
	protoInfoData struct {
		Protodata   []byte
		ID          int32
		SessionTime float32
		RecordStamp time.Time
	}
	snapRaw struct {
		RecordStamp   time.Time
		SessionTime   float32
		TimeOfDay     float32
		AirTemp       float32
		TrackTemp     float32
		TrackWetness  int
		Precipitation float32
		Protodata     []byte
	}
)

var _ api.SpeedmapRepository = (*repo)(nil)

func NewSpeedmapRepository(conn bob.Executor) api.SpeedmapRepository {
	return &repo{
		conn: conn,
	}
}

func (r *repo) Create(
	ctx context.Context,
	rsInfoID int,
	speedmap *racestatev1.PublishSpeedmapRequest,
) error {
	binaryMessage, err := proto.Marshal(speedmap)
	if err != nil {
		return err
	}
	_, err = models.SpeedmapProtos.Insert(
		&models.SpeedmapProtoSetter{
			RSInfoID:  omit.From(int32(rsInfoID)),
			Protodata: omit.From(binaryMessage),
		},
	).One(ctx, r.getExecutor(ctx))
	return err
}

func (r *repo) LoadLatest(
	ctx context.Context,
	eventID int,
) (*racestatev1.PublishSpeedmapRequest, error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.SpeedmapProtos.Columns.Protodata,
		),
		sm.From(models.SpeedmapProtos.Name()),
		models.SelectJoins.SpeedmapProtos.InnerJoin.RSInfo,
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
		sm.Limit(1),
		sm.OrderBy(models.RSInfos.Columns.ID).Desc(),
	)
	res, err := bob.All(
		ctx, r.getExecutor(ctx),
		q, scan.StructMapper[protoInfoData]())
	if err != nil {
		return nil, err
	}
	racestate := &racestatev1.PublishSpeedmapRequest{}
	if err := proto.Unmarshal(res[0].Protodata, racestate); err != nil {
		return nil, err
	}
	return racestate, nil
}

// loads a range of entities starting at startTS
// Note: sessionNum is ignored
func (r *repo) LoadRange(
	ctx context.Context,
	eventID int,
	startTS time.Time,
	limit int,
) (*util.RangeContainer[racestatev1.PublishSpeedmapRequest], error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.SpeedmapProtos.Columns.Protodata,
		),
		sm.From(models.SpeedmapProtos.Name()),
		models.SelectJoins.SpeedmapProtos.InnerJoin.RSInfo,
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
		models.SelectWhere.RSInfos.RecordStamp.GTE(startTS),
		sm.Limit(limit),
		sm.OrderBy(models.RSInfos.Columns.ID).Asc(),
	)
	return r.loadRange(ctx, q, limit)
}

// loads a range of entities starting at sessionTime with session sessionNum
func (r *repo) LoadRangeBySessionTime(
	ctx context.Context,
	eventID int,
	sessionNum uint32,
	sessionTime float64,
	limit int,
) (*util.RangeContainer[racestatev1.PublishSpeedmapRequest], error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.SpeedmapProtos.Columns.Protodata,
		),
		sm.From(models.SpeedmapProtos.Name()),
		models.SelectJoins.SpeedmapProtos.InnerJoin.RSInfo,
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
		models.SelectWhere.RSInfos.SessionNum.EQ(int32(sessionNum)),
		models.SelectWhere.RSInfos.SessionTime.GTE(decimal.NewFromFloat(sessionTime)),
		sm.Limit(limit),
		sm.OrderBy(models.RSInfos.Columns.ID).Asc(),
	)
	return r.loadRange(ctx, q, limit)
}

// loads a range of entities starting at rsInfoId
// sessionNum is ignored
func (r *repo) LoadRangeByID(
	ctx context.Context,
	eventID int,
	rsInfoID int,
	limit int,
) (*util.RangeContainer[racestatev1.PublishSpeedmapRequest], error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.SpeedmapProtos.Columns.Protodata,
		),
		sm.From(models.SpeedmapProtos.Name()),
		models.SelectJoins.SpeedmapProtos.InnerJoin.RSInfo,
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
		models.SelectWhere.RSInfos.ID.GTE(int32(rsInfoID)),
		sm.Limit(limit),
		sm.OrderBy(models.RSInfos.Columns.ID).Asc(),
	)
	return r.loadRange(ctx, q, limit)
}

// loads a range of entities starting at rsInfoId within a session
func (r *repo) LoadRangeByIDWithinSession(
	ctx context.Context,
	eventID int,
	sessionNum uint32,
	rsInfoID int,
	limit int,
) (*util.RangeContainer[racestatev1.PublishSpeedmapRequest], error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.SpeedmapProtos.Columns.Protodata,
		),
		sm.From(models.SpeedmapProtos.Name()),
		models.SelectJoins.SpeedmapProtos.InnerJoin.RSInfo,
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
		models.SelectWhere.RSInfos.ID.GTE(int32(rsInfoID)),
		models.SelectWhere.RSInfos.SessionNum.EQ(int32(sessionNum)),
		sm.Limit(limit),
		sm.OrderBy(models.RSInfos.Columns.ID).Asc(),
	)
	return r.loadRange(ctx, q, limit)
}

//nolint:lll,funlen // readability
func (r *repo) LoadSnapshots(
	ctx context.Context,
	eventID int,
	intervalSecs int,
) ([]*analysisv1.SnapshotData, error) {
	rsID, startTS, err := r.findStartingPoint(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if startTS == nil {
		return []*analysisv1.SnapshotData{}, nil
	}
	log.Debug("found starting point",
		log.Int("rsID", rsID),
		log.Time("startTS", *startTS),
	)
	_ = rsID

	// collects rsInfo of the speedmaps
	subRS := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.RecordStamp,
			psql.F("to_timestamp", psql.Arg(startTS.UnixMilli()))().As("race_start")),
		sm.From(models.RSInfos.Name()),
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
		models.SelectWhere.RSInfos.RecordStamp.GTE(*startTS),
		models.SelectJoins.RSInfos.InnerJoin.SpeedmapProtos,
		sm.OrderBy(models.RSInfos.Columns.RecordStamp).Asc(),
	)

	// creates date_bins of above subquery subRS.
	// we are interedted in the rsInfo.id of the first entry in each bin
	startBins := psql.Select(
		sm.Columns(psql.F("first_value", "sub.id")(
			fm.Over(
				wm.PartitionBy(
					psql.F("date_bin",
						psql.Arg(fmt.Sprintf("'%d seconds'", intervalSecs)),
						"sub.record_stamp",
						"sub.race_start"),
				),
				wm.OrderBy(psql.F("date_bin",
					psql.Arg(fmt.Sprintf("'%d seconds'", intervalSecs)),
					"sub.record_stamp",
					"sub.race_start")),
			),
		).As("firstId")),
		sm.From(subRS).As("sub"),
	)

	// for each startBin entry we load the speedmap along with the rsInfo data
	// this will produce the same query as in LoadSnapshotsLegacy
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.RSInfos.Columns.TimeOfDay,
			models.RSInfos.Columns.AirTemp,
			models.RSInfos.Columns.TrackTemp,
			models.RSInfos.Columns.TrackWetness,
			models.RSInfos.Columns.Precipitation,
			models.SpeedmapProtos.Columns.Protodata,
		),
		sm.From(models.SpeedmapProtos.Name()),
		models.SelectJoins.SpeedmapProtos.InnerJoin.RSInfo,
		sm.Where(models.SpeedmapProtos.Columns.RSInfoID.In(startBins)),

		sm.OrderBy(models.RSInfos.Columns.RecordStamp).Asc(),
	)
	res, err := bob.All(ctx, r.getExecutor(ctx), q, scan.StructMapper[snapRaw]())
	if err != nil {
		return nil, err
	}
	log.Debug("loaded data",
		log.Int("count", len(res)))
	return r.snapRowToSnapshotData(res)
}

// uses raw SQL query to load the snapshots
// this is used until the bob query is implemented
// afterwards we keep it to have a sample of how to transfer complexer SQL to bob
//
//nolint:lll,funlen // readability
func (r *repo) LoadSnapshotsLegacy(
	ctx context.Context,
	eventID int,
	intervalSecs int,
) ([]*analysisv1.SnapshotData, error) {
	// this is supposed to break right now
	rsID, startTS, err := r.findStartingPoint(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if startTS == nil {
		return []*analysisv1.SnapshotData{}, nil
	}

	// Note: we keep the original SQL here for reference
	// reason is to have a reference where parameters are used
	// the raw query for bob can only use the ? argument which may be hard to understand
	originalSQL := `
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
	`
	_ = originalSQL
	// params: eventID, fmt.Sprintf("'%d seconds'", intervalSecs), startTS.UnixMilli(), rsID)

	sqlRaw := `
SELECT rs.record_stamp,rs.session_time, rs.time_of_day, rs.air_temp, rs.track_temp, rs.track_wetness, rs.precipitation, sp.protodata
FROM rs_info rs
  JOIN speedmap_proto sp ON sp.rs_info_id = rs.id
WHERE rs.id IN (SELECT y.firstId
                FROM (SELECT FIRST_VALUE(x.id) OVER (PARTITION BY date_bin (?,x.cur,x.raceStart) ORDER BY date_bin (?,x.cur,x.raceStart)) AS firstId
                             FROM (SELECT rs.id,
                                          rs.record_stamp AS cur,
                                          TO_TIMESTAMP(?) AS raceStart
                                   FROM rs_info rs
                                     JOIN speedmap_proto sp ON sp.rs_info_id = rs.id
                                   WHERE rs.event_id = ? and rs.id >= ?
                                   ORDER BY record_stamp) x) y
                ORDER BY y.firstId)
order by rs.record_stamp
		`
	binParam := fmt.Sprintf("'%d seconds'", intervalSecs)
	// we have to put the parameters in the correct order when just using the generic ? as placeholder
	// this is restriction by bob due to compatibility with other databases
	q := psql.RawQuery(sqlRaw,
		psql.Arg(binParam),
		psql.Arg(binParam),
		psql.Arg(startTS.UnixMilli()),
		psql.Arg(eventID),
		psql.Arg(rsID),
	)

	res, err := bob.All(
		ctx, r.getExecutor(ctx),
		q, scan.StructMapper[snapRaw]())
	if err != nil {
		return nil, err
	}
	log.Debug("loaded snapshots",
		log.Int("count", len(res)),
		log.Int("eventID", eventID),
	)
	return r.snapRowToSnapshotData(res)
}

func (r *repo) snapRowToSnapshotData(
	rawData []snapRaw,
) ([]*analysisv1.SnapshotData, error) {
	ret := make([]*analysisv1.SnapshotData, 0, len(rawData))
	for i := range rawData {
		snapRaw := rawData[i]
		sd := &analysisv1.SnapshotData{
			RecordStamp:   timestamppb.New(snapRaw.RecordStamp),
			SessionTime:   snapRaw.SessionTime,
			TimeOfDay:     uint32(snapRaw.TimeOfDay),
			AirTemp:       snapRaw.AirTemp,
			TrackTemp:     snapRaw.TrackTemp,
			TrackWetness:  commonv1.TrackWetness(snapRaw.TrackWetness),
			Precipitation: snapRaw.Precipitation,
		}
		speedmap := &racestatev1.PublishSpeedmapRequest{}
		if err := proto.Unmarshal(snapRaw.Protodata, speedmap); err != nil {
			return nil, err
		}
		sd.CarClassLaptimes = make(map[string]float32)
		for k, car := range speedmap.Speedmap.Data {
			sd.CarClassLaptimes[k] = car.Laptime
		}
		ret = append(ret, sd)
	}
	return ret, nil
}

// returns the rsInfo.id,record_stamp of first speedmap entry that has a laptime > 0
func (r *repo) findStartingPoint(
	ctx context.Context,
	eventID int,
) (int, *time.Time, error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.SpeedmapProtos.Columns.Protodata,
		),
		sm.From(models.SpeedmapProtos.Name()),
		models.SelectJoins.SpeedmapProtos.InnerJoin.RSInfo,
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
		sm.OrderBy(models.RSInfos.Columns.RecordStamp).Asc(),
	)
	res, err := bob.All(
		ctx, r.getExecutor(ctx),
		q, scan.StructMapper[protoInfoData]())
	if err != nil {
		return 0, nil, err
	}
	for i := range res {
		speedmap := &racestatev1.PublishSpeedmapRequest{}
		if err := proto.Unmarshal(res[i].Protodata, speedmap); err != nil {
			return 0, nil, err
		}
		found := false
		for _, car := range speedmap.Speedmap.Data {
			found = found || car.Laptime > 0
		}
		if found {
			return int(res[i].ID), &res[i].RecordStamp, nil
		}
	}
	return 0, nil, nil
}

// deletes all entries for an event from the database, returns number of rows deleted.
func (r *repo) DeleteByEventID(ctx context.Context, eventID int) (ret int, err error) {
	subQuery := psql.Select(
		sm.Columns(models.RSInfos.Columns.ID),
		sm.From(models.RSInfos.Name()),
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
	)
	var num int64
	num, err = models.SpeedmapProtos.Delete(
		dm.Where(models.SpeedmapProtos.Columns.RSInfoID.In(subQuery)),
	).Exec(ctx, r.getExecutor(ctx))
	if err != nil {
		return 0, err
	}

	return int(num), err
}

// loadRange loads a range of racestate entries from the database
// queryProvider must include the following columns in the following order:
// protodata, record_stamp, session_time, id
func (r *repo) loadRange(
	ctx context.Context,
	q bob.Query,
	limit int,
) (*util.RangeContainer[racestatev1.PublishSpeedmapRequest], error) {
	res, err := bob.All(
		ctx, r.getExecutor(ctx),
		q, scan.StructMapper[protoInfoData]())
	if err != nil {
		return nil, err
	}

	ret := util.RangeContainer[racestatev1.PublishSpeedmapRequest]{
		Data: make([]*racestatev1.PublishSpeedmapRequest, 0, limit),
	}
	for i := range res {
		speedmap := &racestatev1.PublishSpeedmapRequest{}
		if err := proto.Unmarshal(res[i].Protodata, speedmap); err != nil {
			return nil, err
		}
		ret.Data = append(ret.Data, speedmap)

		ret.LastTimestamp = res[i].RecordStamp
		ret.LastSessionTime = res[i].SessionTime
		ret.LastRsInfoID = int(res[i].ID)
	}

	return &ret, nil
}

func (r *repo) getExecutor(ctx context.Context) bob.Executor {
	if executor := bobCtx.FromContext(ctx); executor != nil {
		return executor
	}
	return r.conn
}
