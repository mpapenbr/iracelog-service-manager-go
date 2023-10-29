package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gammazero/nexus/v3/client"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/analysis"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/track"
)

type ProviderService struct {
	pool   *pgxpool.Pool
	Lookup ProviderLookup
}

type (
	ProviderLookup map[string]*ProviderData
	ProviderData   struct {
		Event        *model.DbEvent
		Analysis     *model.DbAnalysis
		Registered   time.Time
		Clients      []*Client // currently not used
		ActiveClient *Client
		Processor    *processing.Processor
		StopChan     chan struct{} // used to stop background tasks
	}
	// contains informations about the connected Provider Client
	Client struct {
		WampClient *client.Client
		CancelFunc context.CancelFunc
		// chan for state message processor
	}
)

var providerService ProviderService

func InitProviderService(pool *pgxpool.Pool) *ProviderService {
	providerService = ProviderService{pool: pool, Lookup: ProviderLookup{}}
	return &providerService
}

//nolint:whitespace //can't make both the linter and editor happy :(
func (s *ProviderService) RegisterEvent(req *RegisterEventRequest) (
	*ProviderData, error,
) {
	if data, ok := s.Lookup[req.EventKey]; ok {
		return data, nil
	}
	dbEvent, err := s.registerEventInDb(req)
	if err != nil {
		return nil, err
	}
	data := &ProviderData{
		Event:      dbEvent,
		Registered: time.Now(),
		StopChan:   make(chan struct{}),
	}
	s.Lookup[req.EventKey] = data
	return data, nil
}

//nolint:whitespace //can't make both the linter and editor happy :(
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

	newEvent := model.DbEvent{
		Key: req.EventKey, Name: req.EventInfo.Name, Description: req.EventInfo.Description,
		Data: model.EventData{Info: req.EventInfo, Manifests: req.Manifests},
	}
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

func (s *ProviderService) StoreEventExtra(entry *model.DbEventExtra) error {
	return pgx.BeginFunc(context.Background(), s.pool, func(tx pgx.Tx) error {
		var trackEntry *model.DbTrack

		if err := event.CreateExtra(tx.Conn(), entry); err != nil {
			return err
		}
		trackEntry, err := track.LoadById(tx.Conn(), entry.Data.Track.ID)
		if err != nil {
			return err
		}
		if trackEntry.Data.Pit == nil &&
			entry.Data.Track.Pit != nil &&
			entry.Data.Track.Pit.LaneLength > 0 {

			trackEntry.Data.Pit = entry.Data.Track.Pit
			_, err := track.Update(tx.Conn(), trackEntry)
			return err
		}

		return nil
	})
}

func (s *ProviderService) UpdateAnalysisData(eventKey string) error {
	return pgx.BeginFunc(context.Background(), s.pool, func(tx pgx.Tx) error {
		providerData, ok := s.Lookup[eventKey]
		if !ok {
			return nil
		}
		var newData model.AnalysisDataGeneric
		inRec, _ := json.Marshal(providerData.Processor.CurrentData)
		err := json.Unmarshal(inRec, &newData)
		if err != nil {
			return err
		}

		if providerData.Analysis == nil {
			newEntry := model.DbAnalysis{
				EventID: providerData.Event.ID,
				Data:    newData,
			}
			providerData.Analysis, err = analysis.Create(tx.Conn(), &newEntry)
			return err
		} else {
			providerData.Analysis.Data = newData
			if _, err := analysis.Update(tx.Conn(), providerData.Analysis); err != nil {
				return err
			}
		}
		return nil
	})
}
