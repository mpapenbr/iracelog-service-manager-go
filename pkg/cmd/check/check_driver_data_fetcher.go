package check

import (
	"context"
	"os"
	"strconv"
	"time"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	csRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/car/proto"
	eventrepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/util"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewCheckDataFetcherCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "datafetcher eventId",
		Short: "check conversion of database entries to gRPC model (dev only)",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			checkDriverDataFetcher(args[0])
		},
	}
	cmd.Flags().Float64Var(&startTS, "start", 0, "start timestamp")
	return cmd
}

func checkDriverDataFetcher(eventArg string) {
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
	df := initDriverDataFetcher(pool, eventId, time.Time{}, 50)
	i := 0
	for {
		data := df.next()
		if data == nil {
			break
		}

		log.Debug("driver data", log.Int("i", i))
		i++
	}
}

//nolint:lll // readablity
func initDriverDataFetcher(pool *pgxpool.Pool, eventId int, lastTS time.Time, limit int) myFetcher[racestatev1.PublishDriverDataRequest] {
	df := &commonFetcher[racestatev1.PublishDriverDataRequest]{
		lastTS: lastTS,
		loader: func(startTs time.Time) (*util.RangeContainer[racestatev1.PublishDriverDataRequest], error) {
			return csRepo.LoadRange(context.Background(), pool, eventId, startTs, limit)
		},
	}

	return df
}

type myLoaderFunc[E any] func(startTs time.Time) (*util.RangeContainer[E], error)

type myFetcher[E any] interface {
	next() *E
}

//nolint:unused // false positive
type commonFetcher[E any] struct {
	loader myLoaderFunc[E]
	buffer []*E
	lastTS time.Time
}

//nolint:unused // false positive
func (f *commonFetcher[E]) next() *E {
	if len(f.buffer) == 0 {
		f.fetch()
	}
	if len(f.buffer) == 0 {
		return nil
	}
	ret := f.buffer[0]
	f.buffer = f.buffer[1:]

	return ret
}

//nolint:unused // false positive
func (f *commonFetcher[E]) fetch() {
	var err error
	c, err := f.loader(f.lastTS)
	if err != nil {
		log.Fatal("error loading data", log.ErrorField(err))
	}
	f.buffer = c.Data
	log.Debug("loaded data", log.Int("count", len(f.buffer)))
}
