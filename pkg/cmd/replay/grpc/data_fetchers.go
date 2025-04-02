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
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/util"
)

//
//nolint:lll,whitespace // readablity, editor/linter
func initDriverDataFetcher(
	pool *pgxpool.Pool,
	eventID int,
	lastTS time.Time,
	limit int,
) myFetcher[racestatev1.PublishDriverDataRequest] {
	df := &commonFetcher[racestatev1.PublishDriverDataRequest]{
		lastTS: lastTS,
		loader: func(startTs time.Time) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error) {
			return csRepo.LoadRange(context.Background(), pool, eventID, startTs, limit)
		},
	}

	return df
}

//nolint:lll,whitespace // readablity, editor/linter
func initStateDataFetcher(
	pool *pgxpool.Pool,
	eventID int,
	lastTS time.Time,
	limit int,
) myFetcher[racestatev1.PublishStateRequest] {
	df := &commonFetcher[racestatev1.PublishStateRequest]{
		lastTS: lastTS,
		loader: func(startTs time.Time) (*util.RangeContainer[racestatev1.PublishStateRequest], error) {
			return rsRepo.LoadRange(context.Background(), pool, eventID, startTs, limit)
		},
	}

	return df
}

//nolint:lll,whitespace // readablity, editor/linter
func initSpeedmapDataFetcher(
	pool *pgxpool.Pool,
	eventID int,
	lastTS time.Time,
	limit int,
) myFetcher[racestatev1.PublishSpeedmapRequest] {
	df := &commonFetcher[racestatev1.PublishSpeedmapRequest]{
		lastTS: lastTS,
		loader: func(startTs time.Time) (*util.RangeContainer[racestatev1.PublishSpeedmapRequest], error) {
			return smRepo.LoadRange(context.Background(), pool, eventID, startTs, limit)
		},
	}

	return df
}

type myFetcher[E any] interface {
	next() *E
}

type myLoaderFunc[E any] func(startTs time.Time) (*util.RangeContainer[E], error)

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
	c, err := f.loader(f.lastTS)
	if err != nil {
		log.Fatal("error loading data", log.ErrorField(err))
	}
	f.buffer = c.Data
	log.Debug("loaded data", log.Int("count", len(f.buffer)))
}
