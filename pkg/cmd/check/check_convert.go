package check

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/convert"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	eventrepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/event"
	staterepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/state"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewCheckConvertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert eventId",
		Short: "check conversion of database entries to gRPC model (dev only)",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			checkConvert(args[0])
		},
	}
	cmd.Flags().Float64Var(&startTS, "start", 0, "start timestamp")
	return cmd
}

var startTS float64

func checkConvert(eventArg string) {
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
	eventId, _ := strconv.Atoi(eventArg)
	ctx := context.Background()
	postgresAddr := utils.ExtractFromDBUrl(config.DB)
	if err = utils.WaitForTCP(postgresAddr, timeout); err != nil {
		log.Fatal("database  not ready", log.ErrorField(err))
	}
	pool := postgres.InitWithUrl(config.DB)
	defer pool.Close()
	sourceEvent, err := eventrepo.LoadById(ctx, pool, eventId)
	if err != nil {
		log.Error("event not found", log.ErrorField(err))
		return
	}
	log.Info("event found", log.String("event", sourceEvent.Key))
	df := initDataFetcher(pool, eventId)
	sc := convert.NewStateMessageConverter(sourceEvent, sourceEvent.Key)
	for {
		state := df.next()
		if state == nil {
			break
		}
		x := sc.ConvertStatePayload(&state.Data)
		log.Debug("converted state", log.Any("data", x))
	}
}

func initDataFetcher(pool *pgxpool.Pool, eventId int) *dataFetcher {
	df := &dataFetcher{
		pool:    pool,
		eventId: eventId,
		lastTS:  startTS,
	}

	return df
}

type dataFetcher struct {
	pool   *pgxpool.Pool
	buffer []*model.DbState

	eventId int
	lastTS  float64
}

func (d *dataFetcher) fetch() {
	ctx := context.Background()
	var err error
	d.buffer, err = staterepo.LoadByEventId(ctx, d.pool, d.eventId, d.lastTS, 100)
	if err != nil {
		log.Fatal("error loading data", log.ErrorField(err))
	}
	log.Debug("loaded data", log.Int("count", len(d.buffer)))
}

func (d *dataFetcher) next() *model.DbState {
	if len(d.buffer) == 0 {
		d.fetch()
	}
	if len(d.buffer) == 0 {
		return nil
	}
	ret := d.buffer[0]
	d.buffer = d.buffer[1:]
	d.lastTS = ret.Data.Timestamp
	return ret
}
