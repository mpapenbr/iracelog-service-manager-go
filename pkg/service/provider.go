package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/track"
)

type ProviderService struct {
	pool   *pgxpool.Pool
	active ProviderLookup
}

type (
	ProviderLookup map[string]*ProviderData
	ProviderData   struct {
		Event      *model.DbEvent
		Registered time.Time
	}
)

var providerService ProviderService

func InitProviderService(pool *pgxpool.Pool) *ProviderService {
	providerService = ProviderService{pool: pool, active: ProviderLookup{}}
	return &providerService
}

//nolint:whitespace //can't make bot the linter and editor happy :(
func (s *ProviderService) RegisterEvent(req *RegisterEventRequest) (
	*ProviderData, error,
) {
	if data, ok := s.active[req.EventKey]; ok {
		return data, nil
	}
	dbEvent, err := s.registerEventInDb(req)
	if err != nil {
		return nil, err
	}
	data := &ProviderData{Event: dbEvent, Registered: time.Now()}
	s.active[req.EventKey] = data
	return data, nil
}

//nolint:whitespace //can't make bot the linter and editor happy :(
func (s *ProviderService) registerEventInDb(req *RegisterEventRequest) (
	*model.DbEvent,
	error,
) {
	var result *model.DbEvent
	tx, err := s.pool.Begin(context.Background())
	if err != nil {
		return nil, err
	}
	// Rollback is safe to call even if the tx is already closed, so if
	// the tx commits successfully, this is a no-op
	//nolint:errcheck //no errcheck by design
	defer tx.Rollback(context.Background())

	// if there is already an event registered with that key, return that one
	existing, err := event.LoadByKey(tx.Conn(), req.EventKey)
	if err == nil {
		return existing, nil
	}

	newEvent := model.DbEvent{Key: req.EventKey}
	result, err = event.Create(tx.Conn(), &newEvent)
	if err != nil {
		return nil, err
	}

	_, err = track.LoadById(tx.Conn(), req.EventInfo.TrackId)
	if err != nil {
		err = track.Create(
			tx.Conn(),
			&model.DbTrack{ID: req.EventInfo.TrackId, Data: req.TrackInfo})
		if err != nil {
			return nil, err
		}
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return nil, err
	}

	return result, nil
}
