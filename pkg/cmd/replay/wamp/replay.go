//nolint:funlen // keeping by design
package wamp

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	commonv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/event/v1"
	providerv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/provider/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"
	trackv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/track/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/cmd/replay/util"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/convert"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	carrepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/car"
	eventrepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/event"
	trackrepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/track"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewReplayWampCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wamp eventId",
		Short: "replay an event from database using WAMP data (dev only)",
		Long: `replay an event from database
This command replays an event from the database and sends it to the gRPC endpoints.
Note: This is only for debugging purposes and should not be used in production.
		`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return startReplay(args[0])
		},
	}

	return cmd
}

func startReplay(eventArg string) error {
	logger := log.DevLogger(
		os.Stderr,
		util.ParseLogLevel(util.LogLevel, log.DebugLevel),
		log.WithCaller(true),
		log.AddCallerSkip(1))
	log.ResetDefault(logger)

	// wait for database
	timeout, err := time.ParseDuration(config.WaitForServices)
	if err != nil {
		log.Warn("Invalid duration value. Setting default 60s", log.ErrorField(err))
		timeout = 60 * time.Second
	}
	postgresAddr := utils.ExtractFromDBUrl(config.DB)
	if err = utils.WaitForTCP(postgresAddr, timeout); err != nil {
		log.Fatal("database  not ready", log.ErrorField(err))
	}
	pool := postgres.InitWithUrl(config.DB)
	defer pool.Close()

	eventId, err := strconv.Atoi(eventArg)
	if err != nil {
		return err
	}
	wampReplay, err := newWampReplayTask(pool, eventId)
	if err != nil {
		return err
	}
	log.Info("Starting replay")
	r := util.NewReplayTask(wampReplay)
	if err := r.Replay(eventId); err != nil {
		return err
	}

	log.Info("Replay done")
	return nil
}

type wampReplayTask struct {
	util.ReplayDataProvider
	pool              *pgxpool.Pool
	eventId           int
	sourceTrack       *model.DbTrack
	driverData        []*model.DbCar
	sourceEvent       *model.DbEvent
	driverDataIdx     int
	eventSelector     *commonv1.EventSelector
	stateFetcher      *stateFetcher
	speedmapFetcher   *speedmapFetcher
	stateConverter    *convert.StateConverter
	speedmapConverter *convert.SpeedmapMessageConverter
}

func newWampReplayTask(pool *pgxpool.Pool, eventId int) (*wampReplayTask, error) {
	var err error
	ret := &wampReplayTask{pool: pool, eventId: eventId}

	ret.sourceEvent, err = eventrepo.LoadById(context.Background(), pool, eventId)
	if err != nil {
		return nil, err
	}
	ret.sourceTrack, err = trackrepo.LoadById(context.Background(), pool,
		ret.sourceEvent.Data.Info.TrackId)
	if err != nil {
		return nil, err
	}

	ret.driverData, err = carrepo.LoadByEventId(context.Background(), pool, eventId)
	if err != nil {
		return nil, err
	}
	ret.stateFetcher = initStateFetcher(pool, eventId, 0)
	ret.speedmapFetcher = initSpeedmapFetcher(pool, eventId, 0)

	return ret, nil
}

//nolint:errcheck,lll // keeping by design
func (r *wampReplayTask) ProvideEventData(eventId int) *providerv1.RegisterEventRequest {
	req := r.createRegisterRequest()
	r.eventSelector = &commonv1.EventSelector{Arg: &commonv1.EventSelector_Key{Key: req.Event.Key}}
	r.stateConverter = convert.NewStateMessageConverter(r.sourceEvent, req.Event.Key)
	r.speedmapConverter = convert.NewSpeedmapMessageConverter()
	return req
}

func (r *wampReplayTask) NextDriverData() *racestatev1.PublishDriverDataRequest {
	if r.driverDataIdx < len(r.driverData) {
		item := r.driverData[r.driverDataIdx]
		req := racestatev1.PublishDriverDataRequest{
			Event:          r.eventSelector,
			Cars:           convert.ConvertCarInfos(item.Data.Payload.Cars),
			Entries:        convert.ConvertCarEntries(item.Data.Payload.Entries),
			CarClasses:     convert.ConvertCarClasses(item.Data.Payload.CarClasses),
			SessionTime:    float32(item.Data.Payload.SessionTime),
			Timestamp:      timestamppb.New(time.UnixMilli(int64(item.Data.Timestamp))),
			CurrentDrivers: convert.ConvertCurrentDrivers(item.Data.Payload.CurrentDrivers),
		}
		r.driverDataIdx++
		return &req
	}
	return nil
}

func (r *wampReplayTask) NextStateData() *racestatev1.PublishStateRequest {
	item := r.stateFetcher.next()
	if item == nil {
		return nil
	}
	req := r.stateConverter.ConvertStatePayload(&item.Data)
	return req
}

func (r *wampReplayTask) NextSpeedmapData() *racestatev1.PublishSpeedmapRequest {
	item := r.speedmapFetcher.next()
	if item == nil {
		return nil
	}
	req := racestatev1.PublishSpeedmapRequest{
		Event:     r.eventSelector,
		Speedmap:  r.speedmapConverter.ConvertSpeedmapPayload(item),
		Timestamp: timestamppb.New(time.UnixMilli(int64(item.Timestamp * 1000))),
	}
	return &req
}

//nolint:whitespace // can't make both editor and linter happy
func (r *wampReplayTask) createRegisterRequest() *providerv1.RegisterEventRequest {
	convertSectors := func(secs []model.Sector) []*trackv1.Sector {
		ret := make([]*trackv1.Sector, 0, len(secs))
		for _, s := range secs {
			ret = append(ret, &trackv1.Sector{
				Num:      uint32(s.SectorNum),
				StartPct: float32(s.SectorStartPct),
			})
		}
		return ret
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

	// dbValue is delivered as UTC, but in real it was a CET timestamp
	fixTimestamp := func(dbValue time.Time) time.Time {
		justTime := dbValue.Format("2006-01-02 15:04:05")
		ret, _ := time.Parse("2006-01-02 15:04:05 MST", fmt.Sprintf("%s CET", justTime))
		return ret
	}
	recordingMode := func() providerv1.RecordingMode {
		if util.DoNotPersist {
			return providerv1.RecordingMode_RECORDING_MODE_DO_NOT_PERSIST
		} else {
			return providerv1.RecordingMode_RECORDING_MODE_PERSIST
		}
	}

	if util.EventKey == "" {
		util.EventKey = uuid.New().String()
	}
	e := r.sourceEvent
	t := r.sourceTrack
	req := &providerv1.RegisterEventRequest{
		Event: &eventv1.Event{
			Key:               util.EventKey,
			Description:       e.Description,
			Name:              e.Name,
			EventTime:         timestamppb.New(fixTimestamp(e.RecordStamp)),
			TrackId:           uint32(t.Data.ID),
			RaceloggerVersion: e.Data.Info.RaceloggerVersion,
			TeamRacing:        e.Data.Info.TeamRacing > 0,
			MultiClass:        e.Data.Info.MultiClass,
			NumCarTypes:       uint32(e.Data.Info.NumCarTypes),
			NumCarClasses:     uint32(e.Data.Info.NumCarClasses),
			IrSessionId:       int32(e.Data.Info.IrSessionId),
			Sessions:          convertSessions(e.Data.Info.Sessions),
			ReplayInfo: &eventv1.ReplayInfo{
				MinTimestamp: timestamppb.New(
					time.UnixMilli(int64(e.Data.ReplayInfo.MinTimestamp * 1000))),
				MinSessionTime: float32(e.Data.ReplayInfo.MinSessionTime),
				MaxSessionTime: float32(e.Data.ReplayInfo.MaxSessionTime),
			},

			// PitSpeed:  float32(e.Data.Info.TrackPitSpeed),
		},
		Track: &trackv1.Track{
			Id:        &trackv1.TrackId{Id: uint32(t.Data.ID)},
			Name:      t.Data.Name,
			ShortName: t.Data.ShortName,
			Config:    t.Data.Config,
			Length:    float32(t.Data.Length),
			PitSpeed:  float32(e.Data.Info.TrackPitSpeed),
			Sectors:   convertSectors(t.Data.Sectors),
			PitInfo: &trackv1.PitInfo{
				Entry:      float32(t.Data.Pit.Entry),
				Exit:       float32(t.Data.Pit.Exit),
				LaneLength: float32(t.Data.Pit.LaneLength),
			},
		},
		RecordingMode: recordingMode(),
	}
	return req
}
