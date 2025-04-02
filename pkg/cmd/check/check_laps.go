package check

import (
	"context"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	aRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/analysis/proto"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewDisplayLapsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "laps eventId",
		Short: "display laps (dev only)",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			displayLaps(cmd.Context(), args[0])
		},
	}
	cmd.Flags().Float64Var(&startTS, "start", 0, "start timestamp")

	return cmd
}

func displayLaps(ctx context.Context, eventArg string) {
	logger := log.GetFromContext(ctx).Named("check")
	logger.Info("display laps")
	// wait for database
	timeout, err := time.ParseDuration(config.WaitForServices)
	if err != nil {
		logger.Warn("Invalid duration value. Setting default 60s", log.ErrorField(err))
		timeout = 60 * time.Second
	}
	eventID, _ := strconv.Atoi(eventArg)

	postgresAddr := utils.ExtractFromDBURL(config.DB)
	if err = utils.WaitForTCP(postgresAddr, timeout); err != nil {
		logger.Fatal("database  not ready", log.ErrorField(err))
	}
	pool := postgres.InitWithURL(config.DB)
	defer pool.Close()
	data, err := aRepo.LoadByEventID(ctx, pool, eventID)
	if err != nil {
		logger.Fatal("error loading data", log.ErrorField(err))
	}
	logger.Info("got laps: ", log.Int("count", len(data.CarLaps)))
	for i, d := range data.CarLaps {
		for _, l := range d.Laps {
			logger.Info("lap", log.Int("idx", i), log.String("carNum", d.CarNum),
				log.Any("data", l))
		}
	}
}
