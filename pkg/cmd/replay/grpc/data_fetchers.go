package grpc

import (
	"context"
	"time"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	csRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/car/proto"
	rsRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/racestate"
	smRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/speedmap/proto"
)

//nolint:lll // readablity
func initDriverDataFetcher(pool *pgxpool.Pool, eventId int, lastTS time.Time, limit int) myFetcher[racestatev1.PublishDriverDataRequest] {
	df := &commonFetcher[racestatev1.PublishDriverDataRequest]{
		lastTS: lastTS,
		loader: func(startTs time.Time) ([]*racestatev1.PublishDriverDataRequest, time.Time, error) {
			return csRepo.LoadRange(context.Background(), pool, eventId, startTs, limit)
		},
	}

	return df
}

//nolint:lll // readablity
func initStateDataFetcher(pool *pgxpool.Pool, eventId int, lastTS time.Time, limit int) myFetcher[racestatev1.PublishStateRequest] {
	df := &commonFetcher[racestatev1.PublishStateRequest]{
		lastTS: lastTS,
		loader: func(startTs time.Time) ([]*racestatev1.PublishStateRequest, time.Time, error) {
			return rsRepo.LoadRange(context.Background(), pool, eventId, startTs, limit)
		},
	}

	return df
}

//nolint:lll // readablity
func initSpeedmapDataFetcher(pool *pgxpool.Pool, eventId int, lastTS time.Time, limit int) myFetcher[racestatev1.PublishSpeedmapRequest] {
	df := &commonFetcher[racestatev1.PublishSpeedmapRequest]{
		lastTS: lastTS,
		loader: func(startTs time.Time) ([]*racestatev1.PublishSpeedmapRequest, time.Time, error) {
			return smRepo.LoadRange(context.Background(), pool, eventId, startTs, limit)
		},
	}

	return df
}

type myFetcher[E any] interface {
	next() *E
}

type myLoaderFunc[E any] func(startTs time.Time) ([]*E, time.Time, error)

//nolint:unused // false positive
type commonFetcher[E any] struct {
	loader myLoaderFunc[E]
	buffer []*E
	lastTS time.Time
}

//nolint:unused // false positive
func (f *commonFetcher[E]) next() *E {
	if len(f.buffer) == 0 {
		f.fetch()
	}
	if len(f.buffer) == 0 {
		return nil
	}
	ret := f.buffer[0]
	f.buffer = f.buffer[1:]

	return ret
}

//nolint:unused // false positive
func (f *commonFetcher[E]) fetch() {
	var err error
	f.buffer, f.lastTS, err = f.loader(f.lastTS)
	if err != nil {
		log.Fatal("error loading data", log.ErrorField(err))
	}
	log.Debug("loaded data", log.Int("count", len(f.buffer)))
}
