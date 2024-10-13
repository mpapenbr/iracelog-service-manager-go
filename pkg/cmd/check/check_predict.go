package check

import (
	"context"
	"strconv"
	"time"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/predict"
	aRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/analysis/proto"
	carRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/car/proto"
	eventRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/event"
	rsRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/racestate"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

var (
	sessionTimeArg string
	carNum         string
	stintLaps      int
	stintAvg       float32
)

//nolint:lll // readability
func NewPredictCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "predict eventId",
		Short: "display laps (dev only)",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			predictRace(cmd.Context(), args[0])
		},
	}
	cmd.Flags().StringVar(&sessionTimeArg, "session-time", "", "session time when to predict")
	cmd.Flags().StringVar(&carNum, "car-num", "", "start timestamp")
	cmd.Flags().Float32Var(&stintAvg, "stint-avg", 0, "calc with this average lap time")
	cmd.Flags().IntVar(&stintLaps, "stint-laps", 0, "calc with this laps per stint")

	return cmd
}

type predictData struct {
	carNum      string
	sessionTime time.Duration
	pool        *pgxpool.Pool
	l           *log.Logger
	analyis     *analysisv1.Analysis
	carInfo     *racestatev1.PublishDriverDataRequest
	event       *eventv1.Event
	stintLaps   int
	stintAvg    float32
}

func (pd *predictData) loadData(eventId int) error {
	var err error
	ctx := context.Background()
	if pd.event, err = eventRepo.LoadById(ctx, pd.pool, eventId); err != nil {
		pd.l.Error("error loading event data", log.ErrorField(err))
		return err
	}

	if pd.analyis, err = aRepo.LoadByEventId(ctx, pd.pool, eventId); err != nil {
		pd.l.Error("error loading analysis data", log.ErrorField(err))
		return err
	}
	if pd.carInfo, err = carRepo.LoadLatest(ctx, pd.pool, eventId); err != nil {
		pd.l.Error("error loading car data", log.ErrorField(err))
		return err
	}

	var states []*racestatev1.PublishStateRequest
	if states, _, err = rsRepo.LoadRangeBySessionTime(
		ctx,
		pd.pool,
		eventId,
		pd.sessionTime.Seconds(),
		1); err != nil || len(states) == 0 {
		pd.l.Error("error loading state", log.ErrorField(err))
		return err
	}
	pd.debug()
	pred := predict.NewPrediction(pd.analyis, pd.carInfo, states[0], pd.event, pd.carNum)
	pd.l.Info("predicting", log.Any("data", pred))

	return nil
}

func (pd *predictData) debug() {
	if states, _, err := rsRepo.LoadRangeBySessionTime(
		context.Background(),
		pd.pool,
		int(pd.event.Id),
		float64(pd.event.ReplayInfo.MinSessionTime)-5,
		10); err != nil || len(states) == 0 {
		pd.l.Error("error loading state", log.ErrorField(err))
	} else {
		for _, s := range states {
			pd.l.Debug("state", log.Any("session", s.Session))
		}
	}
}

func (pd *predictData) analyze() {
}

func predictRace(ctx context.Context, eventArg string) {
	logger := log.GetFromContext(ctx).Named("check")
	// wait for database
	timeout, err := time.ParseDuration(config.WaitForServices)
	if err != nil {
		logger.Warn("Invalid duration value. Setting default 60s", log.ErrorField(err))
		timeout = 60 * time.Second
	}
	eventId, _ := strconv.Atoi(eventArg)

	sessionTime, err := time.ParseDuration(sessionTimeArg)
	if err != nil {
		logger.Fatal("Invalid session-time value", log.ErrorField(err))
		return
	}

	postgresAddr := utils.ExtractFromDBUrl(config.DB)
	if err = utils.WaitForTCP(postgresAddr, timeout); err != nil {
		logger.Fatal("database  not ready", log.ErrorField(err))
	}
	pool := postgres.InitWithUrl(config.DB)
	defer pool.Close()
	predictData := &predictData{
		pool:        pool,
		l:           logger,
		sessionTime: sessionTime,
		carNum:      carNum,
		stintLaps:   stintLaps,
		stintAvg:    stintAvg,
	}
	if err := predictData.loadData(eventId); err != nil {
		return
	}

	predictData.analyze()
}
