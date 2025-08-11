package bob

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stephenafamo/bob"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/analysis"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/car"
	carProto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/car/proto"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/event/ext"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/racestate"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/speedmap"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/tenant"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/track"
)

type bobRepositories struct {
	analysisRepository  api.AnalysisRepository
	trackRepository     api.TrackRepository
	tenantRepository    api.TenantRepository
	eventRepository     api.EventRepository
	eventExtRepository  api.EventExtRepository
	carRepository       api.CarRepository
	carProtoRepository  api.CarProtoRepository
	racestateRepository api.RacestateRepository
	speedmapRepository  api.SpeedmapRepository
}

var _ api.Repositories = (*bobRepositories)(nil)

func NewRepositoriesFromPool(pool *pgxpool.Pool) api.Repositories {
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	return NewRepositories(db)
}

func NewRepositories(db bob.DB) api.Repositories {
	return &bobRepositories{
		analysisRepository:  analysis.NewAnalysisRepository(db),
		trackRepository:     track.NewTrackRepository(db),
		tenantRepository:    tenant.NewTenantRepository(db),
		eventRepository:     event.NewEventRepository(db),
		eventExtRepository:  ext.NewEventExtRepository(db),
		carRepository:       car.NewCarRepository(db),
		carProtoRepository:  carProto.NewCarProtoRepository(db),
		racestateRepository: racestate.NewRacestateRepository(db),
		speedmapRepository:  speedmap.NewSpeedmapRepository(db),
	}
}

func (r *bobRepositories) Analysis() api.AnalysisRepository {
	return r.analysisRepository
}

func (r *bobRepositories) Track() api.TrackRepository {
	return r.trackRepository
}

func (r *bobRepositories) Tenant() api.TenantRepository {
	return r.tenantRepository
}

func (r *bobRepositories) Event() api.EventRepository {
	return r.eventRepository
}

func (r *bobRepositories) EventExt() api.EventExtRepository {
	return r.eventExtRepository
}

func (r *bobRepositories) Car() api.CarRepository {
	return r.carRepository
}

func (r *bobRepositories) CarProto() api.CarProtoRepository {
	return r.carProtoRepository
}

func (r *bobRepositories) Racestate() api.RacestateRepository {
	return r.racestateRepository
}

func (r *bobRepositories) Speedmap() api.SpeedmapRepository {
	return r.speedmapRepository
}

func NewTrackRepositoryFromPool(pool *pgxpool.Pool) api.TrackRepository {
	return track.NewTrackRepository(bob.NewDB(stdlib.OpenDBFromPool(pool)))
}
