package grpc

import (
	"context"
	"os"
	"strconv"
	"time"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	providerv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/provider/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/cmd/replay/util"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	eventrepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/event"
	trackrepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/track"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewReplayGrpcCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grpc eventId",
		Short: "replay an event from database using stored grpc data (dev only)",
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

type grpcReplayTask struct {
	util.ReplayDataProvider
	pool        *pgxpool.Pool
	eventId     int
	sourceEvent *eventv1.Event
	sourceTrack *trackv1.Track

	eventSelector     *commonv1.EventSelector
	stateFetcher      myFetcher[racestatev1.PublishStateRequest]
	speedmapFetcher   myFetcher[racestatev1.PublishSpeedmapRequest]
	driverDataFetcher myFetcher[racestatev1.PublishDriverDataRequest]
}

func newWampReplayTask(pool *pgxpool.Pool, eventId int) (*grpcReplayTask, error) {
	var err error
	ret := &grpcReplayTask{pool: pool, eventId: eventId}

	ret.sourceEvent, err = eventrepo.LoadById(context.Background(), pool, eventId)
	if err != nil {
		return nil, err
	}
	ret.sourceTrack, err = trackrepo.LoadById(context.Background(), pool,
		int(ret.sourceEvent.TrackId))
	if err != nil {
		return nil, err
	}

	ret.driverDataFetcher = initDriverDataFetcher(pool, eventId, time.Time{}, 50)
	ret.stateFetcher = initStateDataFetcher(pool, eventId, time.Time{}, 100)
	ret.speedmapFetcher = initSpeedmapDataFetcher(pool, eventId, time.Time{}, 100)

	return ret, nil
}

//nolint:lll // readablity
func (r *grpcReplayTask) ProvideEventData(eventId int) *providerv1.RegisterEventRequest {
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
	r.sourceEvent.Key = util.EventKey
	r.eventSelector = &commonv1.EventSelector{Arg: &commonv1.EventSelector_Key{Key: r.sourceEvent.Key}}
	return &providerv1.RegisterEventRequest{
		Event:         r.sourceEvent,
		Track:         r.sourceTrack,
		RecordingMode: recordingMode(),
	}
}

func (r *grpcReplayTask) NextDriverData() *racestatev1.PublishDriverDataRequest {
	item := r.driverDataFetcher.next()
	if item == nil {
		return nil
	}
	item.Event = r.eventSelector
	return item
}

func (r *grpcReplayTask) NextStateData() *racestatev1.PublishStateRequest {
	item := r.stateFetcher.next()
	if item == nil {
		return nil
	}
	item.Event = r.eventSelector
	return item
}

func (r *grpcReplayTask) NextSpeedmapData() *racestatev1.PublishSpeedmapRequest {
	item := r.speedmapFetcher.next()
	if item == nil {
		return nil
	}
	item.Event = r.eventSelector
	return item
}
