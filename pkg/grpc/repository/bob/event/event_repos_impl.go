package event

import (
	"context"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/shopspring/decimal"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/models"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/mytypes"
	api "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	bobCtx "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/context"
)

type (
	repo struct {
		conn bob.Executor
	}
)

var _ api.EventRepository = (*repo)(nil)

func NewEventRepository(conn bob.Executor) api.EventRepository {
	return &repo{
		conn: conn,
	}
}

//nolint:whitespace,funlen // editor/linter issue
func (r *repo) Create(
	ctx context.Context,
	event *eventv1.Event,
	tenantID uint32,
) error {
	sessions := mytypes.EventSessionSlice{}
	for _, item := range event.Sessions {
		sessions = append(sessions, mytypes.EventSession{
			Num:         int(item.Num),
			Name:        item.Name,
			Laps:        int(item.Laps),
			SessionTime: int(item.SessionTime),
			Type:        int(item.Type),
		})
	}
	tireInfos := mytypes.TireInfoSlice{}
	for _, item := range event.TireInfos {
		tireInfos = append(tireInfos, mytypes.TireInfo{
			Index:        int(item.Index),
			CompoundType: item.CompoundType,
		})
	}

	setter := &models.EventSetter{
		Name:              omit.From(event.Name),
		EventKey:          omit.From(event.Key),
		Description:       omitnull.From(event.Description),
		TenantID:          omit.From(int32(tenantID)),
		EventTime:         omit.From(event.EventTime.AsTime()),
		RaceloggerVersion: omit.From(event.RaceloggerVersion),
		TrackID:           omit.From(int32(event.TrackId)),
		TeamRacing:        omit.From(event.TeamRacing),
		MultiClass:        omit.From(event.MultiClass),
		NumCarTypes:       omit.From(int32(event.NumCarTypes)),
		NumCarClasses:     omit.From(int32(event.NumCarClasses)),
		IrSessionID:       omitnull.From(event.IrSessionId),
		IrSubSessionID:    omit.From(event.IrSubSessionId),
		PitSpeed:          omit.From(decimal.NewFromFloat32(event.PitSpeed)),
		Sessions:          omit.From(sessions),
		TireInfos:         omitnull.From(tireInfos),
	}
	if event.ReplayInfo != nil {
		setter.ReplayMinTimestamp = omit.From(event.ReplayInfo.MinTimestamp.AsTime())
		setter.ReplayMinSessionTime = omit.From(decimal.NewFromFloat32(
			event.ReplayInfo.MinSessionTime))
		setter.ReplayMaxSessionTime = omit.From(decimal.NewFromFloat32(
			event.ReplayInfo.MaxSessionTime))
	}
	res, err := models.Events.Insert(setter).One(ctx, r.getExecutor(ctx))
	if err != nil {
		return err
	}
	event.Id = uint32(res.ID)
	return nil
}

//nolint:whitespace // editor/linter issue
func (r *repo) LoadByID(ctx context.Context, id int) (
	*eventv1.Event, error,
) {
	ret, err := models.Events.Query(
		models.SelectWhere.Events.ID.EQ(int32(id))).
		One(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}
	return r.toPBMessage(ret)
}

//nolint:whitespace // editor/linter issue
func (r *repo) LoadByKey(ctx context.Context, key string) (
	*eventv1.Event, error,
) {
	ret, err := models.Events.Query(
		models.SelectWhere.Events.EventKey.EQ(key)).
		One(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}
	return r.toPBMessage(ret)
}

//nolint:whitespace // editor/linter issue
func (r *repo) LoadAll(ctx context.Context, tenantID *uint32) (
	[]*eventv1.Event, error,
) {
	sqlMods := make(bob.Mods[*dialect.SelectQuery], 0)
	sqlMods = append(sqlMods, sm.OrderBy(models.Events.Columns.EventTime).Desc())
	if tenantID != nil {
		tmp := int32(*tenantID)
		sqlMods = append(sqlMods, models.SelectWhere.Events.TenantID.EQ(tmp))
	}
	query := models.Events.Query(
		sqlMods...,
	)

	data, err := query.All(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}
	ret := make([]*eventv1.Event, 0)

	for i := range data {
		item, err := r.toPBMessage(data[i])
		if err != nil {
			return nil, err
		}
		ret = append(ret, item)
	}
	return ret, nil
}

//nolint:whitespace //can't make both the linter and editor happy :(
func (r repo) UpdateEvent(
	ctx context.Context,
	id int,
	req *eventv1.UpdateEventRequest,
) error {
	setter := &models.EventSetter{}

	if req.Name != "" {
		setter.Name = omit.From(req.Name)
	}
	if req.Description != "" {
		setter.Description = omitnull.From(req.Description)
	}
	if req.Key != "" {
		setter.EventKey = omit.From(req.Key)
	}
	if req.ReplayInfo != nil {
		r.addReplayInfo(req.ReplayInfo, setter)
	}

	_, err := models.Events.Update(
		models.UpdateWhere.Events.ID.EQ(int32(id)),
		setter.UpdateMod(),
	).Exec(ctx, r.getExecutor(ctx))

	return err
}

//nolint:whitespace //can't make both the linter and editor happy :(
func (r repo) UpdateReplayInfo(
	ctx context.Context,
	id int,
	replayInfo *eventv1.ReplayInfo,
) error {
	if replayInfo == nil {
		return nil
	}
	setter := &models.EventSetter{}
	r.addReplayInfo(replayInfo, setter)

	_, err := models.Events.Update(
		setter.UpdateMod(),
		models.UpdateWhere.Events.ID.EQ(int32(id)),
	).Exec(ctx, r.getExecutor(ctx))
	return err
}

// deletes an entry from the database, returns number of rows deleted.
func (r *repo) DeleteByID(ctx context.Context, id int) (int, error) {
	ret, err := models.Events.Delete(
		models.DeleteWhere.Events.ID.EQ(int32(id)),
	).Exec(ctx, r.getExecutor(ctx))
	return int(ret), err
}

//nolint:whitespace //can't make both the linter and editor happy :(
func (r *repo) addReplayInfo(
	replayInfo *eventv1.ReplayInfo,
	setter *models.EventSetter,
) {
	if replayInfo == nil {
		return
	}
	setter.ReplayMinSessionTime = omit.From(decimal.NewFromFloat32(
		replayInfo.MinSessionTime))
	setter.ReplayMaxSessionTime = omit.From(decimal.NewFromFloat32(
		replayInfo.MaxSessionTime))
	if replayInfo.MinTimestamp != nil {
		setter.ReplayMinTimestamp = omit.From(replayInfo.MinTimestamp.AsTime())
	}
}

//nolint:funlen // by design
func (r *repo) toPBMessage(dbEvent *models.Event) (*eventv1.Event, error) {
	var item eventv1.Event

	item.Id = uint32(dbEvent.ID)
	item.Name = dbEvent.Name
	item.Key = dbEvent.EventKey
	item.Description = dbEvent.Description.GetOr("")
	item.EventTime = timestamppb.New(dbEvent.EventTime)
	item.RaceloggerVersion = dbEvent.RaceloggerVersion
	item.TeamRacing = dbEvent.TeamRacing
	item.MultiClass = dbEvent.MultiClass
	item.NumCarTypes = uint32(dbEvent.NumCarTypes)
	item.NumCarClasses = uint32(dbEvent.NumCarClasses)
	item.IrSessionId = dbEvent.IrSessionID.GetOr(0)
	item.IrSubSessionId = dbEvent.IrSubSessionID
	item.TrackId = uint32(dbEvent.TrackID)
	item.PitSpeed = float32(dbEvent.PitSpeed.Abs().InexactFloat64())
	item.ReplayInfo = &eventv1.ReplayInfo{
		MinTimestamp:   timestamppb.New(dbEvent.ReplayMinTimestamp),
		MinSessionTime: float32(dbEvent.ReplayMinSessionTime.InexactFloat64()),
		MaxSessionTime: float32(dbEvent.ReplayMaxSessionTime.InexactFloat64()),
	}

	if dbEvent.Sessions != nil {
		sessions := make([]*eventv1.Session, len(dbEvent.Sessions))
		for i := range dbEvent.Sessions {
			sessions[i] = &eventv1.Session{
				Num:         uint32(dbEvent.Sessions[i].Num),
				Name:        dbEvent.Sessions[i].Name,
				SessionTime: int32(dbEvent.Sessions[i].SessionTime),
				Laps:        int32(dbEvent.Sessions[i].Laps),
				Type:        commonv1.SessionType(dbEvent.Sessions[i].Type),
			}
		}
		item.Sessions = sessions
	}
	// note: TireInfos is optional, so we check if value is present
	if dbEvent.TireInfos.IsValue() {
		tireInfos := make([]*eventv1.TireInfo, len(dbEvent.TireInfos.GetOrZero()))
		for i, tireInfo := range dbEvent.TireInfos.GetOrZero() {
			tireInfos[i] = &eventv1.TireInfo{
				Index:        uint32(tireInfo.Index),
				CompoundType: tireInfo.CompoundType,
			}
		}
		item.TireInfos = tireInfos
	}

	return &item, nil
}

func (r *repo) getExecutor(ctx context.Context) bob.Executor {
	if executor := bobCtx.FromContext(ctx); executor != nil {
		return executor
	}
	return r.conn
}
