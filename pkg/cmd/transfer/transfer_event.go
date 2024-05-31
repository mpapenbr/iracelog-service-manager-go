//nolint:all // temporary code
package transfer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/convert"
	carstateDest "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/car/proto"
	eventDest "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/event"
	racestateDest "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/racestate"
	speedmapDest "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/speedmap/proto"
	eventService "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/service/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"

	carSource "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/car"
	eventSource "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/event"
	speedmapSource "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/speedmap"
	stateSource "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/state"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

func NewTransferEventCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event",
		Short: "transfer an event from the old format to the new format",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			transferEvent(args[0])
		},
	}

	return cmd
}

func transferEvent(eventArg string) {
	logger := log.DevLogger(
		os.Stderr,
		parseLogLevel(logLevel, log.InfoLevel),
		log.WithCaller(true),
		log.AddCallerSkip(1))
	log.ResetDefault(logger)
	setupDatabases()
	defer poolSource.Close()
	defer poolDest.Close()

	var err error
	var source *model.DbEvent
	var dest *eventv1.Event
	eventService := eventService.NewEventService(poolDest)

	eventId, _ := strconv.Atoi(eventArg)
	log.Debug("Test", log.Int("event", eventId))
	log.Info("Transfer event", log.Int("event", eventId))
	ctx := context.Background()
	// check if event exists (by eventKey)
	source, err = eventSource.LoadById(ctx, poolSource, eventId)
	if err != nil {
		log.Error("error loading event from source", log.ErrorField(err))
		return
	}

	dest, err = eventDest.LoadByKey(ctx, poolDest, source.Key)
	if !errors.Is(err, pgx.ErrNoRows) {
		log.Info("event exists in destination, deleting")

		err := eventService.DeleteEvent(context.Background(), int(dest.Id))
		if err != nil {
			log.Error("error deleting event in destination", log.ErrorField(err))
			return
		}

	}
	convertSessions := func(sessions []model.EventSession) []*eventv1.Session {
		ret := make([]*eventv1.Session, 0, len(sessions))
		for _, s := range sessions {
			ret = append(ret, &eventv1.Session{
				Num:  uint32(s.Num),
				Name: s.Name,
			})
		}
		return ret
	}

	newEvent := &eventv1.Event{
		Key:               source.Key,
		Description:       source.Description,
		Name:              source.Name,
		EventTime:         timestamppb.New(fixTimestamp(source.RecordStamp)),
		TrackId:           uint32(source.Data.Info.TrackId),
		RaceloggerVersion: source.Data.Info.RaceloggerVersion,
		TeamRacing:        source.Data.Info.TeamRacing > 0,
		MultiClass:        source.Data.Info.MultiClass,
		NumCarTypes:       uint32(source.Data.Info.NumCarTypes),
		NumCarClasses:     uint32(source.Data.Info.NumCarClasses),
		IrSessionId:       int32(source.Data.Info.IrSessionId),
		Sessions:          convertSessions(source.Data.Info.Sessions),
		// PitSpeed:  float32(e.Data.Info.TrackPitSpeed),
	}

	err = pgx.BeginFunc(ctx, poolDest, func(tx pgx.Tx) error {
		if err := eventDest.Create(ctx, tx, newEvent); err != nil {
			log.Error("error creating event", log.ErrorField(err))
			return err
		}

		if err := transferRaceStates(ctx, source, newEvent, tx); err != nil {
			log.Error("error on transferRaceStates", log.ErrorField(err))
			return err
		}
		if err := transferCarStates(ctx, source, newEvent, tx); err != nil {
			log.Error("error on transferCarStates", log.ErrorField(err))
			return err
		}
		if err := transferSpeedmaps(ctx, source, newEvent, tx); err != nil {
			log.Error("error on transferSpeedmaps", log.ErrorField(err))
			return err
		}
		return nil
	})
	log.Info("event transfer complete")
}

func transferCarStates(ctx context.Context, source *model.DbEvent, newEvent *eventv1.Event, tx pgx.Tx) error {
	states, err := carSource.LoadByEventId(ctx, poolSource, source.ID)
	log.Debug("car states", log.Int("states", len(states)))
	if err != nil {
		log.Error("error loading car states from source", log.ErrorField(err))
		return err
	}
	if len(states) > 0 {
		for i := range states {
			item := states[i]
			data := &racestatev1.PublishDriverDataRequest{
				Event:          &commonv1.EventSelector{Arg: &commonv1.EventSelector_Key{Key: newEvent.Key}},
				Cars:           convert.ConvertCarInfos(item.Data.Payload.Cars),
				Entries:        convert.ConvertCarEntries(item.Data.Payload.Entries),
				CarClasses:     convert.ConvertCarClasses(item.Data.Payload.CarClasses),
				SessionTime:    float32(item.Data.Payload.SessionTime),
				Timestamp:      timestamppb.New(time.UnixMilli(int64(item.Data.Timestamp))),
				CurrentDrivers: convert.ConvertCurrentDrivers(item.Data.Payload.CurrentDrivers),
			}
			rsInfoId, err := racestateDest.FindNearestRaceState(ctx, tx, int(newEvent.Id), data.SessionTime)
			if err != nil {
				return err
			}
			carstateDest.Create(ctx, tx, rsInfoId, data)

		}
	}
	return nil
}

func transferSpeedmaps(ctx context.Context, source *model.DbEvent, newEvent *eventv1.Event, tx pgx.Tx) error {
	startTs := 0.0
	goon := true
	speedmapConverter := convert.NewSpeedmapMessageConverter()
	for goon {
		speedmaps, err := speedmapSource.LoadRange(ctx, poolSource, source.ID, startTs, 2000)
		log.Debug("speedmaps", log.Int("speedmaps", len(speedmaps)))
		if err != nil {
			log.Error("error loading states from source", log.ErrorField(err))
			return err
		}
		if len(speedmaps) > 0 {
			for i := range speedmaps {
				req := &racestatev1.PublishSpeedmapRequest{
					Event:     &commonv1.EventSelector{Arg: &commonv1.EventSelector_Key{Key: newEvent.Key}},
					Speedmap:  speedmapConverter.ConvertSpeedmapPayload(speedmaps[i]),
					Timestamp: timestamppb.New(time.UnixMilli(int64(speedmaps[i].Timestamp * 1000))),
				}

				rsInfoId, err := racestateDest.FindNearestRaceState(ctx, tx, int(newEvent.Id), req.Speedmap.SessionTime)
				if err != nil {
					return err
				}
				speedmapDest.Create(ctx, tx, rsInfoId, req)

			}
			startTs = speedmaps[len(speedmaps)-1].Timestamp
			// startTs = math.MaxFloat64
		} else {
			goon = false
		}
	}
	return nil
}

func transferRaceStates(ctx context.Context, source *model.DbEvent, newEvent *eventv1.Event, tx pgx.Tx) error {
	startTs := 0.0
	goon := true
	stateConverter := convert.NewStateMessageConverter(source, source.Key)
	for goon {
		states, err := stateSource.LoadByEventId(ctx, poolSource, source.ID, startTs, 2000)
		log.Debug("states", log.Int("states", len(states)))
		if err != nil {
			log.Error("error loading states from source", log.ErrorField(err))
			return err
		}
		if len(states) > 0 {
			for i := range states {
				res := stateConverter.ConvertStatePayload(&states[i].Data)
				if res != nil {
					if _, err := racestateDest.CreateRaceState(ctx, tx, int(newEvent.Id), res); err != nil {
						return err
					}
				}
			}
			startTs = states[len(states)-1].Data.Timestamp
			// startTs = math.MaxFloat64
		} else {
			goon = false
		}
	}
	return nil
}

// dbValue is delivered as UTC, but in real it was a CET timestamp
func fixTimestamp(dbValue time.Time) time.Time {
	justTime := dbValue.Format("2006-01-02 15:04:05")
	ret, _ := time.Parse("2006-01-02 15:04:05 MST", fmt.Sprintf("%s CET", justTime))
	return ret
}
