package check

import (
	"context"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	stateRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/racestate"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewCheckStateInoutlapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inout eventId",
		Short: "check marker of in/out lap (dev only)",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			checkStateInoutMarker(cmd.Context(), args[0])
		},
	}
	cmd.Flags().Float64Var(&startTS, "start", 0, "start timestamp")

	return cmd
}

func checkStateInoutMarker(ctx context.Context, eventArg string) {
	logger := log.GetFromContext(ctx).Named("check")
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
	states, _ := stateRepo.LoadRange(ctx, pool, eventID, time.Time{}, 1500)
	logger.Info("got racestates: ", log.Int("count", len(states.Data)))
	for i, d := range states.Data {
		for _, c := range d.Cars {
			if c.TimeInfo != nil {
				logger.Info("car", log.Int("idx", i),
					log.Int("carIdx", int(c.CarIdx)),
					log.Int("lc", int(c.Lc)),
					log.Any("timeinfo", c.TimeInfo))
			}
		}
	}
}
