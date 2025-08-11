package api

import (
	"context"
	"errors"
	"time"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	tenantv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/tenant/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/util"
)

var ErrNoRows = errors.New("no rows in result set")

type Repositories interface {
	Analysis() AnalysisRepository
	Track() TrackRepository
	Tenant() TenantRepository
	Event() EventRepository
	EventExt() EventExtRepository
	Car() CarRepository
	CarProto() CarProtoRepository
	Racestate() RacestateRepository
	Speedmap() SpeedmapRepository
}
type AnalysisRepository interface {
	Upsert(
		ctx context.Context,
		eventID int,
		extra *analysisv1.Analysis,
	) error
	LoadByEventID(ctx context.Context, eventID int) (*analysisv1.Analysis, error)
	LoadByEventKey(ctx context.Context, eventKey string) (*analysisv1.Analysis, error)
	DeleteByEventID(ctx context.Context, eventID int) (int, error)
}

type TrackRepository interface {
	Create(ctx context.Context, track *trackv1.Track) error
	LoadByID(ctx context.Context, id int) (*trackv1.Track, error)
	LoadAll(ctx context.Context) ([]*trackv1.Track, error)
	EnsureTrack(ctx context.Context, track *trackv1.Track) error
	UpdatePitInfo(ctx context.Context, id int, pitInfo *trackv1.PitInfo) (int, error)
	DeleteByID(ctx context.Context, id int) (int, error)
}
type TenantRepository interface {
	Create(ctx context.Context, tenant *tenantv1.CreateTenantRequest) (
		*model.Tenant, error,
	)
	LoadAll(ctx context.Context) ([]*model.Tenant, error)
	LoadByID(ctx context.Context, id uint32) (*model.Tenant, error)
	LoadByExternalID(ctx context.Context, externalID string) (*model.Tenant, error)
	LoadByAPIKey(ctx context.Context, apiKey string) (*model.Tenant, error)
	LoadByName(ctx context.Context, name string) (*model.Tenant, error)
	LoadByEventID(ctx context.Context, eventID int) (*model.Tenant, error)
	LoadBySelector(ctx context.Context, sel *commonv1.TenantSelector) (
		*model.Tenant, error,
	)
	Update(ctx context.Context, id uint32, tenant *tenantv1.UpdateTenantRequest) (
		*model.Tenant, error,
	)
	DeleteByID(ctx context.Context, id uint32) (int, error)
}

type EventRepository interface {
	Create(ctx context.Context, event *eventv1.Event, tenantID uint32) error
	LoadByID(ctx context.Context, id int) (*eventv1.Event, error)
	LoadByKey(ctx context.Context, key string) (*eventv1.Event, error)
	LoadAll(ctx context.Context, tenantID *uint32) ([]*eventv1.Event, error)
	UpdateEvent(ctx context.Context, id int, req *eventv1.UpdateEventRequest) error
	UpdateReplayInfo(ctx context.Context, id int, req *eventv1.ReplayInfo) error
	DeleteByID(ctx context.Context, id int) (int, error)
}
type EventExtRepository interface {
	Upsert(
		ctx context.Context,
		eventID int,
		extra *racestatev1.ExtraInfo,
	) error
	LoadByEventID(ctx context.Context, eventID int) (*racestatev1.ExtraInfo, error)
	DeleteByEventID(ctx context.Context, eventID int) (int, error)
}
type CarRepository interface {
	Create(
		ctx context.Context,
		eventID int,
		driverstate *racestatev1.PublishDriverDataRequest,
	) error

	DeleteByEventID(ctx context.Context, eventID int) (int, error)
}

// Repository for car state data stored as protobuf messages.
type CarProtoRepository interface {
	Create(
		ctx context.Context,
		eventID int,
		driverstate *racestatev1.PublishDriverDataRequest,
	) error
	LoadLatest(
		ctx context.Context,
		eventID int,
	) (*racestatev1.PublishDriverDataRequest, error)
	LoadRange(
		ctx context.Context,
		eventID int,
		startTS time.Time,
		limit int,
	) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error)
	LoadRangeBySessionTime(
		ctx context.Context,
		eventID int,
		sessionNum uint32,
		sessionTime float64,
		limit int,
	) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error)
	LoadRangeByID(
		ctx context.Context,
		eventID int,
		rsInfoID int,
		limit int,
	) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error)

	LoadRangeByIDWithinSession(
		ctx context.Context,
		eventID int,
		sessionNum uint32,
		rsInfoID int,
		limit int,
	) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error)
	DeleteByEventID(ctx context.Context, eventID int) (int, error)
}

// Repository for race state data stored as protobuf messages.
type RacestateRepository interface {
	CreateRacestate(
		ctx context.Context,
		eventID int,
		racestate *racestatev1.PublishStateRequest,
	) (int, error)
	// use this if FindNearestRacestate yields no result
	CreateDummyRacestateInfo(
		ctx context.Context,
		eventID int,
		ts time.Time,
		sessionTime float32,
		sessionNum uint32,
	) (rsInfoID int, err error)
	FindNearestRacestate(
		ctx context.Context,
		eventID int,
		sessionTime float32,
	) (rsInfoID int, err error)
	LoadLatest(
		ctx context.Context,
		eventID int,
	) (*racestatev1.PublishStateRequest, error)
	LoadRange(
		ctx context.Context,
		eventID int,
		startTS time.Time,
		limit int,
	) (*util.RangeContainer[racestatev1.PublishStateRequest], error)
	LoadRangeBySessionTime(
		ctx context.Context,
		eventID int,
		sessionNum uint32,
		sessionTime float64,
		limit int,
	) (*util.RangeContainer[racestatev1.PublishStateRequest], error)
	LoadRangeByID(
		ctx context.Context,
		eventID int,
		rsInfoID int,
		limit int,
	) (*util.RangeContainer[racestatev1.PublishStateRequest], error)

	LoadRangeByIDWithinSession(
		ctx context.Context,
		eventID int,
		sessionNum uint32,
		rsInfoID int,
		limit int,
	) (*util.RangeContainer[racestatev1.PublishStateRequest], error)
	CollectMessages(
		ctx context.Context,
		eventID int,
	) ([]*racestatev1.MessageContainer, error)
	DeleteByEventID(ctx context.Context, eventID int) (int, error)
}

// Repository for speedmap data stored as protobuf messages.
type SpeedmapRepository interface {
	Create(
		ctx context.Context,
		eventID int,
		speedmap *racestatev1.PublishSpeedmapRequest,
	) error

	LoadLatest(
		ctx context.Context,
		eventID int,
	) (*racestatev1.PublishSpeedmapRequest, error)
	LoadRange(
		ctx context.Context,
		eventID int,
		startTS time.Time,
		limit int,
	) (*util.RangeContainer[racestatev1.PublishSpeedmapRequest], error)
	LoadRangeBySessionTime(
		ctx context.Context,
		eventID int,
		sessionNum uint32,
		sessionTime float64,
		limit int,
	) (*util.RangeContainer[racestatev1.PublishSpeedmapRequest], error)
	LoadRangeByID(
		ctx context.Context,
		eventID int,
		rsInfoID int,
		limit int,
	) (*util.RangeContainer[racestatev1.PublishSpeedmapRequest], error)

	LoadRangeByIDWithinSession(
		ctx context.Context,
		eventID int,
		sessionNum uint32,
		rsInfoID int,
		limit int,
	) (*util.RangeContainer[racestatev1.PublishSpeedmapRequest], error)
	LoadSnapshots(
		ctx context.Context,
		eventID int,
		intervalSecs int,
	) ([]*analysisv1.SnapshotData, error)
	LoadSnapshotsLegacy(
		ctx context.Context,
		eventID int,
		intervalSecs int,
	) ([]*analysisv1.SnapshotData, error)

	DeleteByEventID(ctx context.Context, eventID int) (int, error)
}

type TransactionManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}
