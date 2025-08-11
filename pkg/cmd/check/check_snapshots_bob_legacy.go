package check

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
	"github.com/stephenafamo/bob"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/speedmap"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewCheckSnapshotsBobLegacyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot-bob-legacy eventId",
		Short: "check snapshot retrieval (dev only)",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			checkSnapshotBobLegacy(args[0])
		},
	}
	cmd.Flags().Float64Var(&startTS, "start", 0, "start timestamp")
	return cmd
}

func checkSnapshotBobLegacy(eventArg string) {
	// wait for database
	timeout, err := time.ParseDuration(config.WaitForServices)
	if err != nil {
		log.Warn("Invalid duration value. Setting default 60s", log.ErrorField(err))
		timeout = 60 * time.Second
	}
	eventID, _ := strconv.Atoi(eventArg)
	ctx := context.Background()
	postgresAddr := utils.ExtractFromDBURL(config.DB)
	if err = utils.WaitForTCP(postgresAddr, timeout); err != nil {
		log.Fatal("database  not ready", log.ErrorField(err))
	}
	pool := postgres.InitWithURL(config.DB)
	defer pool.Close()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	smRepo := speedmap.NewSpeedmapRepository(db)
	data, _ := smRepo.LoadSnapshotsLegacy(ctx, eventID, 300)
	log.Info("got snapshots: ", log.Int("count", len(data)))
	for i, d := range data {
		log.Info("snapshot", log.Int("idx", i), log.String("data", d.String()))
	}
}
