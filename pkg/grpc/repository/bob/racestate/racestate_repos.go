//nolint:whitespace // can't make both editor and linter happy
package racestate

import (
	"context"
	"errors"
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
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/dbinfo"
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

var _ api.RacestateRepository = (*repo)(nil)

func NewRacestateRepository(conn bob.Executor) api.RacestateRepository {
	return &repo{
		conn: conn,
	}
}

// stores the racestate info and the protobuf message in the database
// returns the id of the created rs_info entry for further use
//
//nolint:funlen // long function
func (r *repo) CreateRacestate(
	ctx context.Context,
	eventID int,
	racestate *racestatev1.PublishStateRequest,
) (rsInfoID int, err error) {
	setter := &models.RSInfoSetter{
		EventID:       omit.From(int32(eventID)),
		RecordStamp:   omit.From(racestate.Timestamp.AsTime()),
		SessionTime:   omit.From(decimal.NewFromFloat32(racestate.Session.SessionTime)),
		SessionNum:    omit.From(int32(racestate.Session.SessionNum)),
		TimeOfDay:     omit.From(int32(racestate.Session.TimeOfDay)),
		AirTemp:       omit.From(decimal.NewFromFloat32(racestate.Session.AirTemp)),
		TrackTemp:     omit.From(decimal.NewFromFloat32(racestate.Session.TrackTemp)),
		TrackWetness:  omit.From(int32(racestate.Session.TrackWetness)),
		Precipitation: omit.From(decimal.NewFromFloat32(racestate.Session.Precipitation)),
	}
	var res *models.RSInfo
	res, err = models.RSInfos.Insert(setter).One(ctx, r.getExecutor(ctx))
	if err != nil {
		return 0, err
	}
	rsInfoID = int(res.ID)

	binaryMessage, err := proto.Marshal(racestate)
	if err != nil {
		return 0, err
	}
	_, err = models.RaceStateProtos.Insert(
		&models.RaceStateProtoSetter{
			RSInfoID:  omit.From(int32(rsInfoID)),
			Protodata: omit.From(binaryMessage),
		},
	).One(ctx, r.getExecutor(ctx))
	if err != nil {
		return 0, err
	}

	for _, msg := range racestate.Messages {
		binaryMessage, err := proto.Marshal(msg)
		if err != nil {
			return 0, err
		}
		_, err = models.MSGStateProtos.Insert(
			&models.MSGStateProtoSetter{
				RSInfoID:  omit.From(int32(rsInfoID)),
				Protodata: omit.From(binaryMessage),
			},
		).One(ctx, r.getExecutor(ctx))
		if err != nil {
			return 0, err
		}

	}
	return rsInfoID, nil
}

// this method is used if there are driver data prior to the first race state
func (r *repo) CreateDummyRacestateInfo(
	ctx context.Context,
	eventID int,
	ts time.Time,
	sessionTime float32,
	sessionNum uint32,
) (rsInfoID int, err error) {
	setter := &models.RSInfoSetter{
		EventID:       omit.From(int32(eventID)),
		RecordStamp:   omit.From(ts),
		SessionTime:   omit.From(decimal.NewFromFloat32(sessionTime)),
		SessionNum:    omit.From(int32(sessionNum)),
		TimeOfDay:     omit.From(int32(0)),
		AirTemp:       omit.From(decimal.NewFromFloat32(0)),
		TrackTemp:     omit.From(decimal.NewFromFloat32(0)),
		TrackWetness:  omit.From(int32(0)),
		Precipitation: omit.From(decimal.NewFromFloat32(0)),
	}
	var res *models.RSInfo
	res, err = models.RSInfos.Insert(setter).One(ctx, r.getExecutor(ctx))
	if err != nil {
		return 0, err
	}
	rsInfoID = int(res.ID)
	return rsInfoID, nil
}

// Note: we may not need this method anymore
func (r *repo) FindNearestRacestate(
	ctx context.Context,
	eventID int,
	sessionTime float32,
) (rsInfoID int, err error) {
	var res models.RSInfoSlice
	res, err = models.RSInfos.Query(
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
		models.SelectWhere.RSInfos.SessionTime.LTE(decimal.NewFromFloat32(sessionTime)),
		sm.OrderBy(dbinfo.RSInfos.Columns.SessionTime.Name).Desc(),
		sm.Limit(1),
	).All(ctx, r.getExecutor(ctx))
	if err != nil {
		return 0, err
	}

	if len(res) == 0 {
		res, err = models.RSInfos.Query(
			models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
			sm.OrderBy(dbinfo.RSInfos.Columns.SessionTime.Name).Asc(),
			sm.Limit(1),
		).All(ctx, r.getExecutor(ctx))
		if err != nil {
			return 0, err
		}
		if len(res) == 0 {
			return 0, errors.New("no race state found")
		}
	}
	rsInfoID = int(res[0].ID)
	return rsInfoID, nil
}

func (r *repo) LoadLatest(
	ctx context.Context,
	eventID int,
) (*racestatev1.PublishStateRequest, error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.RaceStateProtos.Columns.Protodata,
		),
		sm.From(models.RaceStateProtos.Name()),
		models.SelectJoins.RaceStateProtos.InnerJoin.RSInfo,
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
	racestate := &racestatev1.PublishStateRequest{}
	if err := proto.Unmarshal(res[0].Protodata, racestate); err != nil {
		return nil, err
	}
	return racestate, nil
}

func (r *repo) CollectMessages(
	ctx context.Context,
	eventID int,
) ([]*racestatev1.MessageContainer, error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.MSGStateProtos.Columns.Protodata,
		),
		sm.From(models.MSGStateProtos.Name()),
		models.SelectJoins.MSGStateProtos.InnerJoin.RSInfo,
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
		sm.OrderBy(models.RSInfos.Columns.ID).Asc(),
	)
	res, err := bob.All(
		ctx, r.getExecutor(ctx),
		q, scan.StructMapper[protoInfoData]())
	if err != nil {
		return nil, err
	}

	ret := make([]*racestatev1.MessageContainer, 0)
	for i := range res {
		msg := &racestatev1.Message{}
		if err := proto.Unmarshal(res[i].Protodata, msg); err != nil {
			return nil, err
		}
		ret = append(ret, &racestatev1.MessageContainer{
			SessionTime: res[i].SessionTime,
			Timestamp:   timestamppb.New(res[i].RecordStamp),
			Message:     msg,
		})
	}
	return ret, nil
}

func (r *repo) LoadRange(
	ctx context.Context,
	eventID int,
	startTS time.Time,
	limit int,
) (*util.RangeContainer[racestatev1.PublishStateRequest], error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.RaceStateProtos.Columns.Protodata,
		),
		sm.From(models.RaceStateProtos.Name()),
		models.SelectJoins.RaceStateProtos.InnerJoin.RSInfo,
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
) (*util.RangeContainer[racestatev1.PublishStateRequest], error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.RaceStateProtos.Columns.Protodata,
		),
		sm.From(models.RaceStateProtos.Name()),
		models.SelectJoins.RaceStateProtos.InnerJoin.RSInfo,
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
) (*util.RangeContainer[racestatev1.PublishStateRequest], error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.RaceStateProtos.Columns.Protodata,
		),
		sm.From(models.RaceStateProtos.Name()),
		models.SelectJoins.RaceStateProtos.InnerJoin.RSInfo,
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
) (*util.RangeContainer[racestatev1.PublishStateRequest], error) {
	q := psql.Select(
		sm.Columns(
			models.RSInfos.Columns.ID,
			models.RSInfos.Columns.SessionTime,
			models.RSInfos.Columns.RecordStamp,
			models.RaceStateProtos.Columns.Protodata,
		),
		sm.From(models.RaceStateProtos.Name()),
		models.SelectJoins.RaceStateProtos.InnerJoin.RSInfo,
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
		models.SelectWhere.RSInfos.ID.GTE(int32(rsInfoID)),
		models.SelectWhere.RSInfos.SessionNum.EQ(int32(sessionNum)),
		sm.Limit(limit),
		sm.OrderBy(models.RSInfos.Columns.ID).Asc(),
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
) (*util.RangeContainer[racestatev1.PublishStateRequest], error) {
	res, err := bob.All(
		ctx, r.getExecutor(ctx),
		q, scan.StructMapper[protoInfoData]())
	if err != nil {
		return nil, err
	}

	ret := util.RangeContainer[racestatev1.PublishStateRequest]{
		Data: make([]*racestatev1.PublishStateRequest, 0, limit),
	}
	for i := range res {
		racesate := &racestatev1.PublishStateRequest{}
		if err := proto.Unmarshal(res[i].Protodata, racesate); err != nil {
			return nil, err
		}
		ret.Data = append(ret.Data, racesate)

		ret.LastTimestamp = res[i].RecordStamp
		ret.LastSessionTime = res[i].SessionTime
		ret.LastRsInfoID = int(res[i].ID)
	}

	return &ret, nil
}

// deletes all entries for an event from the database, returns number of rows deleted.
//
//nolint:lll // readability
func (r *repo) DeleteByEventID(ctx context.Context, eventID int) (ret int, err error) {
	subQuery := psql.Select(
		sm.Columns(dbinfo.RSInfos.Columns.ID.Name),
		sm.From(dbinfo.RSInfos.Name),
		models.SelectWhere.RSInfos.EventID.EQ(int32(eventID)),
	)
	_, err = models.RaceStateProtos.Delete(
		dm.Where(models.RaceStateProtos.Columns.RSInfoID.In(subQuery)),
	).Exec(ctx, r.getExecutor(ctx))
	if err != nil {
		return 0, err
	}
	_, err = models.MSGStateProtos.Delete(
		dm.Where(models.MSGStateProtos.Columns.RSInfoID.In(subQuery)),
	).Exec(ctx, r.getExecutor(ctx))
	if err != nil {
		return 0, err
	}
	var num int64
	num, err = models.RSInfos.Delete(
		models.DeleteWhere.RSInfos.EventID.EQ(int32(eventID)),
	).Exec(ctx, r.getExecutor(ctx))
	return int(num), err
}

func (r *repo) getExecutor(ctx context.Context) bob.Executor {
	if executor := bobCtx.FromContext(ctx); executor != nil {
		return executor
	}
	return r.conn
}
