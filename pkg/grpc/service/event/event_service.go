package event

import (
	"context"

	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
)

type EventService struct {
	repos api.Repositories
	txMgr api.TransactionManager
}

//nolint:whitespace // editor/linter issue
func NewEventService(
	repos api.Repositories,
	txMgr api.TransactionManager,
) *EventService {
	return &EventService{repos: repos, txMgr: txMgr}
}

func (s *EventService) DeleteEvent(ctx context.Context, eventID int) error {
	return s.txMgr.RunInTx(ctx, func(ctx context.Context) error {
		//nolint:govet // false positive
		var err error
		var num int

		num, err = s.repos.Analysis().DeleteByEventID(ctx, eventID)
		if err != nil {
			return err
		}
		log.Debug("Deleted analysis", log.Int("num", num))

		num, err = s.repos.Speedmap().DeleteByEventID(ctx, eventID)
		if err != nil {
			return err
		}
		log.Debug("Deleted speedmaps", log.Int("num", num))

		num, err = s.repos.CarProto().DeleteByEventID(ctx, eventID)
		if err != nil {
			return err
		}
		log.Debug("Deleted car states", log.Int("num", num))

		num, err = s.repos.Racestate().DeleteByEventID(ctx, eventID)
		if err != nil {
			return err
		}
		log.Debug("Deleted racestates", log.Int("num", num))

		num, err = s.repos.Car().DeleteByEventID(ctx, eventID)
		if err != nil {
			return err
		}
		log.Debug("Deleted c_car* data", log.Int("num", num))

		num, err = s.repos.EventExt().DeleteByEventID(ctx, eventID)
		if err != nil {
			return err
		}
		log.Debug("Deleted event_ext data", log.Int("num", num))

		_, err = s.repos.Event().DeleteByID(ctx, eventID)
		return err
	})
}

//nolint:whitespace // can't make both editor and linter happy
func (s *EventService) UpdateEvent(
	ctx context.Context,
	eventID int,
	req *eventv1.UpdateEventRequest,
) (*eventv1.Event, error) {
	if err := s.txMgr.RunInTx(ctx, func(ctx context.Context) error {
		return s.repos.Event().UpdateEvent(ctx, eventID, req)
	}); err != nil {
		return nil, err
	}
	return s.repos.Event().LoadByID(ctx, eventID)
}

func (s *EventService) GetSnapshotData(ctx context.Context, eventID int) (int, error) {
	return 0, nil
}
