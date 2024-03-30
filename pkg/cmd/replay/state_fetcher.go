package replay

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	staterepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/state"
)

func initStateFetcher(pool *pgxpool.Pool, eventId int, startTS float64) *stateFetcher {
	df := &stateFetcher{
		pool:    pool,
		eventId: eventId,
		lastTS:  startTS,
	}

	return df
}

type stateFetcher struct {
	pool    *pgxpool.Pool
	buffer  []*model.DbState
	eventId int
	lastTS  float64
}

func (f *stateFetcher) fetch() {
	ctx := context.Background()
	var err error
	f.buffer, err = staterepo.LoadByEventId(ctx, f.pool, f.eventId, f.lastTS, 100)
	if err != nil {
		log.Fatal("error loading data", log.ErrorField(err))
	}
	log.Debug("loaded data", log.Int("count", len(f.buffer)))
}

func (f *stateFetcher) next() *model.DbState {
	if len(f.buffer) == 0 {
		f.fetch()
	}
	if len(f.buffer) == 0 {
		return nil
	}
	ret := f.buffer[0]
	f.buffer = f.buffer[1:]
	f.lastTS = ret.Data.Timestamp
	return ret
}
