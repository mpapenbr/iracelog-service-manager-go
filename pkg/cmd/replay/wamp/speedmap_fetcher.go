package wamp

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	speedmaprepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/speedmap"
)

//nolint:lll // readablity
func initSpeedmapFetcher(pool *pgxpool.Pool, eventId int, startTS float64) *speedmapFetcher {
	df := &speedmapFetcher{
		pool:    pool,
		eventId: eventId,
		lastTS:  startTS,
	}

	return df
}

type speedmapFetcher struct {
	pool    *pgxpool.Pool
	buffer  []*model.SpeedmapData
	eventId int
	lastTS  float64
}

func (f *speedmapFetcher) fetch() {
	ctx := context.Background()
	var err error
	f.buffer, err = speedmaprepo.LoadRange(ctx, f.pool, f.eventId, f.lastTS, 50)
	if err != nil {
		log.Fatal("error loading data", log.ErrorField(err))
	}
	log.Debug("loaded data", log.Int("count", len(f.buffer)))
}

func (f *speedmapFetcher) next() *model.SpeedmapData {
	if len(f.buffer) == 0 {
		f.fetch()
	}
	if len(f.buffer) == 0 {
		return nil
	}
	ret := f.buffer[0]
	f.buffer = f.buffer[1:]
	f.lastTS = ret.Timestamp
	return ret
}
