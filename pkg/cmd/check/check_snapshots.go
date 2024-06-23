package check

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/speedmap/proto"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewCheckSnapshotsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot eventId",
		Short: "check snapshot retrieval (dev only)",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			checkSnapshot(args[0])
		},
	}
	cmd.Flags().Float64Var(&startTS, "start", 0, "start timestamp")
	return cmd
}

func checkSnapshot(eventArg string) {
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
	data, _ := proto.LoadSnapshots(ctx, pool, eventId, 300)
	log.Info("got snapshots: ", log.Int("count", len(data)))
	for i, d := range data {
		log.Info("snapshot", log.Int("idx", i), log.String("data", d.String()))
	}
}
