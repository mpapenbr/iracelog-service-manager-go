package replay

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	providerv1connect "buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/provider/v1/providerv1connect"
	racestatev1connect "buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/racestate/v1/racestatev1connect"
	eventv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/event/v1"
	providerv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/provider/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"
	trackv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/track/v1"
	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/convert"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	carrepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/car"
	eventrepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/event"
	trackrepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/track"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewReplayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "replay eventId",
		Short: "replay an event from database (dev only)",
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

	cmd.Flags().IntVar(&speed, "speed", 1,
		"Recording speed (<=0 means: go as fast as possible)")
	cmd.Flags().StringVar(&addr, "addr", "localhost:8084", "gRPC server address")
	cmd.Flags().StringVar(&logLevel,
		"log-level",
		"info",
		"controls the log level (debug, info, warn, error, fatal)")
	cmd.Flags().StringVarP(&token,
		"token", "t", "", "authentication token")
	cmd.Flags().StringVar(&eventKey,
		"key", "", "event key to use for replay")
	return cmd
}

var (
	speed    = 1
	addr     = "http://localhost:8084"
	logLevel = "info"
	token    = ""
	eventKey = ""
)

func parseLogLevel(l string, defaultVal log.Level) log.Level {
	level, err := log.ParseLevel(l)
	if err != nil {
		return defaultVal
	}
	return level
}

func startReplay(eventArg string) error {
	logger := log.DevLogger(
		os.Stderr,
		parseLogLevel(logLevel, log.DebugLevel),
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

	r := &replayTask{pool: pool}
	eventId, err := strconv.Atoi(eventArg)
	if err != nil {
		return err
	}
	log.Info("Starting replay")
	if err := r.ReplayEvent(eventId); err != nil {
		return err
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	select {
	case v := <-sigChan:
		log.Debug("Got signal ", log.Any("signal", v))
	case <-r.ctx.Done():
		log.Debug("Context done")
	}

	//nolint:all // keeping by design
	// select {
	// case <-sigChan:
	// 	log.Logger.Debug("Got signal")
	// 	pm.Shutdown()
	// }

	log.Info("Replay done")
	return nil
}

type replayTask struct {
	pool             *pgxpool.Pool
	ctx              context.Context
	cancel           context.CancelFunc
	providerService  providerv1connect.ProviderServiceClient
	raceStateService racestatev1connect.RaceStateServiceClient
	event            *eventv1.Event
	sourceEvent      *model.DbEvent
	wg               sync.WaitGroup
}

func (r *replayTask) ReplayEvent(eventId int) error {
	r.providerService = providerv1connect.NewProviderServiceClient(http.DefaultClient, addr, connect.WithGRPC())
	r.raceStateService = racestatev1connect.NewRaceStateServiceClient(http.DefaultClient, addr, connect.WithGRPC())
	r.ctx, r.cancel = context.WithCancel(context.Background())
	defer r.cancel()

	var sourceTrack *model.DbTrack
	var err error
	r.sourceEvent, err = eventrepo.LoadById(r.ctx, r.pool, eventId)
	if err != nil {
		return err
	}
	sourceTrack, err = trackrepo.LoadById(r.ctx, r.pool, r.sourceEvent.Data.Info.TrackId)
	if err != nil {
		return err
	}

	if r.event, err = r.registerEvent(r.sourceEvent, sourceTrack); err != nil {
		return err
	}
	log.Debug("Event registered", log.String("key", r.event.Key))

	log.Debug("Replaying event", log.String("event", r.sourceEvent.Name))

	r.wg = sync.WaitGroup{}
	r.wg.Add(3)
	go r.sendDriverData()
	go r.sendStateData()
	go r.sendSpeedmapData()

	log.Debug("Waiting for tasks to finish")
	r.wg.Wait()
	log.Debug("About to unregister event")
	err = r.unregisterEvent()
	log.Debug("Event unregistered", log.String("key", r.event.Key))

	return err
}

func (r *replayTask) sendDriverData() error {
	defer r.wg.Done()
	data, err := carrepo.LoadByEventId(r.ctx, r.pool, r.sourceEvent.ID)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	i := 0
	timer := time.NewTimer(1 * time.Second)
	for {
		select {
		case <-timer.C:
			item := data[i]
			req := connect.NewRequest[racestatev1.PublishDriverDataRequest](
				&racestatev1.PublishDriverDataRequest{
					Event:          r.buildEventSelector(),
					Cars:           convert.ConvertCarInfos(item.Data.Payload.Cars),
					Entries:        convert.ConvertCarEntries(item.Data.Payload.Entries),
					CarClasses:     convert.ConvertCarClasses(item.Data.Payload.CarClasses),
					SessionTime:    float32(item.Data.Payload.SessionTime),
					Timestamp:      timestamppb.New(time.UnixMilli(int64(item.Data.Timestamp))),
					CurrentDrivers: convert.ConvertCurrentDrivers(item.Data.Payload.CurrentDrivers),
				},
			)
			r.withToken(req.Header())
			if _, err = r.raceStateService.PublishDriverData(r.ctx, req); err != nil {
				return err
			}

			i++
			if i < len(data) {
				realWait := (data[i].Data.Timestamp - item.Data.Timestamp)
				var wait time.Duration
				if speed > 0 {
					wait = time.Duration(realWait / float64(speed))
				} else {
					wait = 0
				}
				log.Debug("Wait until next send",
					log.String("component", "driverData"),
					log.Duration("realWait", time.Duration(realWait*float64(time.Millisecond))),
					log.Duration("wait", time.Duration(wait)*time.Millisecond))
				timer.Reset(time.Duration(wait) * time.Millisecond)
			} else {
				return nil
			}
		case <-r.ctx.Done():
			log.Debug("Context done")
			return nil
		}
	}
}

func (r *replayTask) sendStateData() error {
	defer r.wg.Done()
	sf := initStateFetcher(r.pool, r.sourceEvent.ID, 0)

	conv := convert.NewStateMessageConverter(r.sourceEvent, r.event.Key)
	i := 0
	timer := time.NewTimer(1 * time.Second)
	lastTS := -1.0
	for {
		select {
		case <-timer.C:
			item := sf.next()
			if item == nil {
				return nil
			}
			if lastTS < 0 {
				lastTS = item.Data.Timestamp
			}
			req := connect.NewRequest[racestatev1.PublishStateRequest](
				conv.ConvertStatePayload(&item.Data),
			)
			r.withToken(req.Header())
			if _, err := r.raceStateService.PublishState(r.ctx, req); err != nil {
				return err
			}

			i++

			// strange: this is measured in seconds
			realWait := (item.Data.Timestamp - lastTS) * 1000
			var wait time.Duration
			if speed > 0 {
				wait = time.Duration(realWait / float64(speed))
			} else {
				wait = 0
			}
			lastTS = item.Data.Timestamp
			log.Debug("Wait until next send",
				log.String("component", "state"),
				log.Int("i", i),
				log.Int("item", item.ID),
				log.Duration("realWait", time.Duration(realWait*float64(time.Millisecond))),
				log.Duration("wait", wait*time.Millisecond))
			timer.Reset(wait * time.Millisecond)

		case <-r.ctx.Done():
			log.Debug("sendStateData Context done")
			return nil
		}
	}
}

func (r *replayTask) sendSpeedmapData() error {
	defer r.wg.Done()
	sf := initSpeedmapFetcher(r.pool, r.sourceEvent.ID, 0)

	conv := convert.NewSpeedmapMessageConverter()
	i := 0
	timer := time.NewTimer(1 * time.Second)
	lastTS := -1.0
	for {
		select {
		case <-timer.C:
			item := sf.next()
			if item == nil {
				return nil
			}
			if lastTS < 0 {
				lastTS = item.Timestamp
			}
			req := connect.NewRequest[racestatev1.PublishSpeedmapRequest](
				&racestatev1.PublishSpeedmapRequest{
					Event:     r.buildEventSelector(),
					Speedmap:  conv.ConvertSpeedmapPayload(item),
					Timestamp: timestamppb.New(time.UnixMilli(int64(item.Timestamp * 1000))),
				},
			)
			r.withToken(req.Header())
			if _, err := r.raceStateService.PublishSpeedmap(r.ctx, req); err != nil {
				return err
			}

			i++

			// strange: this is measured in seconds
			realWait := (item.Timestamp - lastTS) * 1000
			var wait time.Duration
			if speed > 0 {
				wait = time.Duration(realWait / float64(speed))
			} else {
				wait = 0
			}
			lastTS = item.Timestamp
			log.Debug("Wait until next send",
				log.String("component", "speedmap"),
				log.Int("i", i),
				log.Duration("realWait", time.Duration(realWait*float64(time.Millisecond))),
				log.Duration("wait", wait*time.Millisecond))
			timer.Reset(wait * time.Millisecond)

		case <-r.ctx.Done():
			log.Debug("sendSpeedmap Context done")
			return nil
		}
	}
}

//nolint:whitespace // can't make both editor and linter happy
func (r *replayTask) registerEvent(e *model.DbEvent, t *model.DbTrack) (
	*eventv1.Event, error,
) {
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
	if eventKey == "" {
		eventKey = uuid.New().String()
	}
	req := connect.NewRequest[providerv1.RegisterEventRequest](
		&providerv1.RegisterEventRequest{
			Event: &eventv1.Event{
				Key:               eventKey,
				Description:       e.Description,
				Name:              e.Name,
				EventTime:         timestamppb.New(e.RecordStamp),
				TrackId:           uint32(t.Data.ID),
				RaceloggerVersion: e.Data.Info.RaceloggerVersion,
				TeamRacing:        e.Data.Info.TeamRacing > 0,
				MultiClass:        e.Data.Info.MultiClass,
				NumCarTypes:       uint32(e.Data.Info.NumCarTypes),
				NumCarClasses:     uint32(e.Data.Info.NumCarClasses),
				IrSessionId:       int32(e.Data.Info.IrSessionId),
				Sessions:          convertSessions(e.Data.Info.Sessions),
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
			},
		})
	r.withToken(req.Header())
	resp, err := r.providerService.RegisterEvent(r.ctx, req)
	if err != nil {
		return nil, err
	} else {
		return resp.Msg.Event, nil
	}
}

func (r *replayTask) unregisterEvent() error {
	req := connect.NewRequest[providerv1.UnregisterEventRequest](
		&providerv1.UnregisterEventRequest{
			EventSelector: r.buildEventSelector(),
		})
	r.withToken(req.Header())

	_, err := r.providerService.UnregisterEvent(r.ctx, req)
	return err
}

func (r *replayTask) withToken(h http.Header) {
	h.Set("api-token", token)
}

func (r *replayTask) buildEventSelector() *eventv1.EventSelector {
	return &eventv1.EventSelector{Arg: &eventv1.EventSelector_Key{Key: r.event.Key}}
}
