package event

import (
	"context"

	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	analysisproto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/analysis/proto"
	carrepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/car"
	carproto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/car/proto"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/event"
	eventextrepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/event/ext"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/racestate"
	speedmapproto "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/speedmap/proto"
)

type EventService struct {
	pool *pgxpool.Pool
}

func NewEventService(pool *pgxpool.Pool) *EventService {
	return &EventService{pool: pool}
}

func (s *EventService) DeleteEvent(ctx context.Context, eventId int) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		//nolint:govet // false positive
		var err error
		var num int

		num, err = analysisproto.DeleteByEventId(ctx, tx.Conn(), eventId)
		if err != nil {
			return err
		}
		log.Debug("Deleted analysis", log.Int("num", num))

		num, err = speedmapproto.DeleteByEventId(ctx, tx.Conn(), eventId)
		if err != nil {
			return err
		}
		log.Debug("Deleted speedmaps", log.Int("num", num))

		num, err = carproto.DeleteByEventId(ctx, tx.Conn(), eventId)
		if err != nil {
			return err
		}
		log.Debug("Deleted car states", log.Int("num", num))

		num, err = racestate.DeleteByEventId(ctx, tx.Conn(), eventId)
		if err != nil {
			return err
		}
		log.Debug("Deleted racestates", log.Int("num", num))

		num, err = carrepos.DeleteByEventId(ctx, tx.Conn(), eventId)
		if err != nil {
			return err
		}
		log.Debug("Deleted c_car* data", log.Int("num", num))

		num, err = eventextrepos.DeleteByEventId(ctx, tx.Conn(), eventId)
		if err != nil {
			return err
		}
		log.Debug("Deleted event_ext data", log.Int("num", num))

		_, err = event.DeleteById(ctx, tx.Conn(), eventId)
		return err
	})
}

//nolint:whitespace // can't make both editor and linter happy
func (s *EventService) UpdateEvent(
	ctx context.Context,
	eventId int,
	req *eventv1.UpdateEventRequest,
) (*eventv1.Event, error) {
	if err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return event.UpdateEvent(ctx, tx.Conn(), eventId, req)
	}); err != nil {
		return nil, err
	}
	return event.LoadById(ctx, s.pool, eventId)
}

func (s *EventService) GetSnapshotData(ctx context.Context, eventId int) (int, error) {
	return 0, nil
}
