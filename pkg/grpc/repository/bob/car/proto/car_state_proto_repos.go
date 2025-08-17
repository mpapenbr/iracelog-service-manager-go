//nolint:whitespace // can't make both editor and linter happy
package proto

import (
	"context"
	"time"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/aarondl/opt/omit"
	"github.com/shopspring/decimal"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/scan"
	"google.golang.org/protobuf/proto"

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
)

var _ api.CarProtoRepository = (*repo)(nil)

func NewCarProtoRepository(conn bob.Executor) api.CarProtoRepository {
	return &repo{
		conn: conn,
	}
}

func (r *repo) Create(
	ctx context.Context,
	rsInfoID int,
	driverstate *racestatev1.PublishDriverDataRequest,
) error {
	binaryMessage, err := proto.Marshal(driverstate)
	if err != nil {
		return err
	}

	_, err = models.CarStateProtos.Insert(
		&models.CarStateProtoSetter{
			RSInfoID:  omit.From(int32(rsInfoID)),
			Protodata: omit.From(binaryMessage),
		},
	).One(ctx, r.getExecutor(ctx))
	return err
}

func (r *repo) LoadLatest(
	ctx context.Context,
	eventID int,
) (*racestatev1.PublishDriverDataRequest, error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.CarStateProtos.Columns.Protodata,
		),
		sm.From(models.CarStateProtos.Name()),
		models.SelectJoins.CarStateProtos.InnerJoin.RSInfo,
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
	driverData := &racestatev1.PublishDriverDataRequest{}
	if err := proto.Unmarshal(res[0].Protodata, driverData); err != nil {
		return nil, err
	}
	return driverData, nil
}

// loads a range of entities starting at startTS
// Note: sessionNum is ignored
func (r *repo) LoadRange(
	ctx context.Context,
	eventID int,
	startTS time.Time,
	limit int,
) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.CarStateProtos.Columns.Protodata,
		),
		sm.From(models.CarStateProtos.Name()),
		models.SelectJoins.CarStateProtos.InnerJoin.RSInfo,
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
) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.CarStateProtos.Columns.Protodata,
		),
		sm.From(models.CarStateProtos.Name()),
		models.SelectJoins.CarStateProtos.InnerJoin.RSInfo,
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
) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.CarStateProtos.Columns.Protodata,
		),
		sm.From(models.CarStateProtos.Name()),
		models.SelectJoins.CarStateProtos.InnerJoin.RSInfo,
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
		models.SelectWhere.RSInfos.ID.GTE(int32(rsInfoID)),
		sm.Limit(limit),
		sm.OrderBy(models.RSInfos.Columns.ID).Asc(),
		sm.OrderBy(models.RSInfos.Columns.RecordStamp).Asc(),
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
) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.CarStateProtos.Columns.Protodata,
		),
		sm.From(models.CarStateProtos.Name()),
		models.SelectJoins.CarStateProtos.InnerJoin.RSInfo,
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
		models.SelectWhere.RSInfos.ID.GTE(int32(rsInfoID)),
		models.SelectWhere.RSInfos.SessionNum.EQ(int32(sessionNum)),
		sm.Limit(limit),
		sm.OrderBy(models.RSInfos.Columns.ID).Asc(),
		sm.OrderBy(models.RSInfos.Columns.RecordStamp).Asc(),
	)
	return r.loadRange(ctx, q, limit)
}

// loadRange loads a range of racestate entries from the database
// queryProvider must include the following columns in the following order:
// protodata, record_stamp, session_time, id
func (r *repo) loadRange(
	ctx context.Context,
	q bob.Query,
	limit int,
) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error) {
	res, err := bob.All(
		ctx, r.getExecutor(ctx),
		q, scan.StructMapper[protoInfoData]())
	if err != nil {
		return nil, err
	}

	ret := util.RangeContainer[racestatev1.PublishDriverDataRequest]{
		Data: make([]*racestatev1.PublishDriverDataRequest, 0, limit),
	}
	for i := range res {
		driverData := &racestatev1.PublishDriverDataRequest{}
		if err := proto.Unmarshal(res[i].Protodata, driverData); err != nil {
			return nil, err
		}
		ret.Data = append(ret.Data, driverData)

		ret.LastTimestamp = res[i].RecordStamp
		ret.LastSessionTime = res[i].SessionTime
		ret.LastRsInfoID = int(res[i].ID)
	}

	return &ret, nil
}

// deletes all car state related entries of an event from the database
func (r *repo) DeleteByEventID(ctx context.Context, eventID int) (int, error) {
	subQuery := psql.Select(
		sm.Columns(models.RSInfos.Columns.ID),
		sm.From(models.RSInfos.Name()),
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
	)
	ret, err := models.CarStateProtos.Delete(
		dm.Where(models.CarStateProtos.Columns.RSInfoID.In(subQuery)),
	).Exec(ctx, r.getExecutor(ctx))
	return int(ret), err
}

func (r *repo) getExecutor(ctx context.Context) bob.Executor {
	if executor := bobCtx.FromContext(ctx); executor != nil {
		return executor
	}
	return r.conn
}
