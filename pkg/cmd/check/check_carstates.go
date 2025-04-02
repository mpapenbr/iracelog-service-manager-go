package check

import (
	"context"
	"fmt"
	"strconv"
	"time"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	eventrepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewCheckCarStatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "carstates eventId",
		Short: "check car states",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			checkCarStates(cmd.Context(), args[0])
		},
	}
	cmd.Flags().Float64Var(&startTS, "start", 0, "start timestamp")
	return cmd
}

func checkCarStates(ctx context.Context, eventArg string) {
	// wait for database
	timeout, err := time.ParseDuration(config.WaitForServices)
	if err != nil {
		log.Warn("Invalid duration value. Setting default 60s", log.ErrorField(err))
		timeout = 60 * time.Second
	}
	eventID, _ := strconv.Atoi(eventArg)

	postgresAddr := utils.ExtractFromDBURL(config.DB)
	if err = utils.WaitForTCP(postgresAddr, timeout); err != nil {
		log.Fatal("database  not ready", log.ErrorField(err))
	}
	pool := postgres.InitWithURL(config.DB)
	defer pool.Close()
	sourceEvent, err := eventrepo.LoadByID(ctx, pool, eventID)
	if err != nil {
		log.Error("event not found", log.ErrorField(err))
		return
	}
	log.Debug("event found", log.String("event", sourceEvent.Key))
	showCombined(pool, eventID)
}

//nolint:funlen // by design
func showCombined(pool *pgxpool.Pool, eventID int) {
	rows, err := pool.Query(context.Background(), `
select cp.protodata, rsp.protodata, ri.record_stamp,ri.session_time,ri.id
from car_state_proto cp
join rs_info ri on ri.id=cp.rs_info_id
join race_state_proto rsp on ri.id=rsp.rs_info_id
where
ri.event_id=$1
order by ri.id asc limit $2
		`,
		eventID, 300,
	)
	if err != nil {
		log.Error("error fetching car states", log.ErrorField(err))
		return
	}
	defer rows.Close()
	for rows.Next() {
		var csProtodata []byte
		var rsProtodata []byte

		var recordStamp time.Time
		var sessionTime float64
		var rsID int
		err = rows.Scan(&csProtodata, &rsProtodata, &recordStamp, &sessionTime, &rsID)
		if err != nil {
			log.Error("error scanning car states", log.ErrorField(err))
			return
		}
		driverstate := &racestatev1.PublishDriverDataRequest{}
		if err := proto.Unmarshal(csProtodata, driverstate); err != nil {
			log.Error("error unmarshalling driverstate", log.ErrorField(err))
			return
		}
		racestate := &racestatev1.PublishStateRequest{}
		if err := proto.Unmarshal(rsProtodata, racestate); err != nil {
			log.Error("error unmarshalling racestate", log.ErrorField(err))
			return
		}
		log.Info("race state", log.Float64("sessionTime", sessionTime),
			log.Uint32("sessionNum (ds)", driverstate.SessionNum),
			log.Uint32("sessionNum (rs)", racestate.Session.SessionNum),
			log.Int("rsId", rsID),
		)
		fmt.Printf("ds: %d rs: %d rsId: %d\n",
			driverstate.SessionNum, racestate.Session.SessionNum, rsID)
	}
}
