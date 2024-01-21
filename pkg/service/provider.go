//nolint:whitespace //can't make both the linter and editor happy :(
package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gammazero/nexus/v3/client"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/analysis"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/track"
)

var meter = otel.Meter("provider-endpoints")

type ProviderService struct {
	pool   *pgxpool.Pool
	Lookup ProviderLookup
}

type MetricRecorder struct {
	Recorder  metric.Float64Histogram
	MsgCouter int
}

type (
	ProviderLookup map[string]*ProviderData
	ProviderData   struct {
		Event            *model.DbEvent
		Analysis         *model.DbAnalysis
		Registered       time.Time
		Clients          []*Client // currently not used
		ActiveClient     *Client
		Processor        *processing.Processor
		StopChan         chan struct{} // used to stop background tasks
		StateRecorder    MetricRecorder
		SpeedmapRecorder MetricRecorder
		AnalysisRecorder MetricRecorder
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

func (s *ProviderService) RegisterEvent(
	ctx context.Context,
	req *RegisterEventRequest) (
	*ProviderData, error,
) {
	if data, ok := s.Lookup[req.EventKey]; ok {
		return data, nil
	}
	dbEvent, err := s.registerEventInDb(ctx, req)
	if err != nil {
		return nil, err
	}

	stateRecorder, _ := meter.Float64Histogram("state_message",
		metric.WithDescription("processing of state message"),
		metric.WithUnit("s"))
	speedmapRecorder, _ := meter.Float64Histogram("speedmap_message",
		metric.WithDescription("processing of speedmap message"),
		metric.WithUnit("s"))
	analysisRecorder, _ := meter.Float64Histogram("analysis_data",
		metric.WithDescription("store analysis data"),
		metric.WithUnit("s"))

	data := &ProviderData{
		Event:            dbEvent,
		Registered:       time.Now(),
		StopChan:         make(chan struct{}),
		StateRecorder:    MetricRecorder{Recorder: stateRecorder, MsgCouter: 0},
		SpeedmapRecorder: MetricRecorder{Recorder: speedmapRecorder, MsgCouter: 0},
		AnalysisRecorder: MetricRecorder{Recorder: analysisRecorder, MsgCouter: 0},
	}
	s.Lookup[req.EventKey] = data
	return data, nil
}

//nolint:funlen //ok
func (s *ProviderService) registerEventInDb(
	ctx context.Context,
	req *RegisterEventRequest) (
	*model.DbEvent,
	error,
) {
	var result *model.DbEvent
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	// Rollback is safe to call even if the tx is already closed, so if
	// the tx commits successfully, this is a no-op
	//nolint:errcheck //no errcheck by design
	defer tx.Rollback(ctx)

	// if there is already an event registered with that key, return that one

	existing, err := event.LoadByKey(ctx, tx.Conn(), req.EventKey)
	if err == nil {
		return existing, nil
	}

	newEvent := model.DbEvent{
		Key: req.EventKey, Name: req.EventInfo.Name, Description: req.EventInfo.Description,
		Data: model.EventData{Info: req.EventInfo, Manifests: req.Manifests},
	}
	result, err = event.Create(ctx, tx.Conn(), &newEvent)
	if err != nil {
		return nil, err
	}

	_, err = track.LoadById(ctx, tx.Conn(), req.EventInfo.TrackId)
	if err != nil {
		err = track.Create(
			ctx,
			tx.Conn(),
			&model.DbTrack{ID: req.EventInfo.TrackId, Data: req.TrackInfo})
		if err != nil {
			return nil, err
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *ProviderService) StoreEventExtra(
	ctx context.Context,
	entry *model.DbEventExtra,
) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var trackEntry *model.DbTrack

		if err := event.CreateExtra(ctx, tx.Conn(), entry); err != nil {
			return err
		}
		trackEntry, err := track.LoadById(ctx, tx.Conn(), entry.Data.Track.ID)
		if err != nil {
			return err
		}
		if trackEntry.Data.Pit == nil &&
			entry.Data.Track.Pit != nil &&
			entry.Data.Track.Pit.LaneLength > 0 {

			trackEntry.Data.Pit = entry.Data.Track.Pit
			_, err := track.Update(ctx, tx.Conn(), trackEntry)
			return err
		}

		return nil
	})
}

func (s *ProviderService) UpdateAnalysisData(
	ctx context.Context,
	eventKey string,
) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
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
			providerData.Analysis, err = analysis.Create(ctx, tx.Conn(), &newEntry)
			return err
		} else {
			providerData.Analysis.Data = newData
			if _, err := analysis.Update(
				ctx,
				tx.Conn(),
				providerData.Analysis); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *ProviderService) UpdateReplayInfo(ctx context.Context, eventKey string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		providerData, ok := s.Lookup[eventKey]
		if !ok {
			return nil
		}

		if _, err := event.UpdateReplayInfo(
			ctx,
			tx.Conn(),
			eventKey,
			providerData.Processor.ReplayInfo); err != nil {
			return err
		}

		return nil
	})
}
