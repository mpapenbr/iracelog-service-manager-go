package issue208

// copies session num to PublishDriverDataRequest
// see issue #208

import (
	"context"
	"time"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	eventrepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewIssue208Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue208",
		Short: "copies session num from state message to driver data message",

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			migrateCarStates(cmd.Context())
		},
	}
	return cmd
}

func migrateCarStates(ctx context.Context) {
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

	if events, err := eventrepo.LoadAll(ctx, pool); err == nil {
		for _, event := range events {
			doMigrateCarStates(pool, event.Id)
		}
	} else {
		log.Error("error loading events", log.ErrorField(err))
	}
}

//nolint:funlen // long function
func doMigrateCarStates(pool *pgxpool.Pool, eventId uint32) {
	updateErr := pgx.BeginFunc(context.Background(), pool, func(tx pgx.Tx) error {
		rows, err := pool.Query(context.Background(), `
select
cp.protodata, rsp.protodata, ri.record_stamp,ri.session_time,ri.id
from car_state_proto cp
join rs_info ri on ri.id=cp.rs_info_id
join race_state_proto rsp on ri.id=rsp.rs_info_id
where
ri.event_id=$1
order by ri.id asc
	`,
			eventId,
		)
		if err != nil {
			log.Error("error fetching car states", log.ErrorField(err))
			return err
		}
		defer rows.Close()
		var updated int64 = 0
		for rows.Next() {
			var csProtodata []byte
			var rsProtodata []byte

			var recordStamp time.Time
			var sessionTime float64
			var rsId int
			err = rows.Scan(&csProtodata, &rsProtodata, &recordStamp, &sessionTime, &rsId)
			if err != nil {
				log.Error("error scanning car states", log.ErrorField(err))
				return err
			}
			driverstate := &racestatev1.PublishDriverDataRequest{}
			if err := proto.Unmarshal(csProtodata, driverstate); err != nil {
				log.Error("error unmarshalling driverstate", log.ErrorField(err))
				return err
			}
			racestate := &racestatev1.PublishStateRequest{}
			if err := proto.Unmarshal(rsProtodata, racestate); err != nil {
				log.Error("error unmarshalling racestate", log.ErrorField(err))
				return err
			}
			log.Debug("race state", log.Float64("sessionTime", sessionTime),
				log.Uint32("sessionNum (ds)", driverstate.SessionNum),
				log.Uint32("sessionNum (rs)", racestate.Session.SessionNum),
				log.Int("rsId", rsId),
			)
			driverstate.SessionNum = racestate.Session.SessionNum
			if binaryMessage, err := proto.Marshal(driverstate); err != nil {
				log.Error("error arshalling driverstate", log.ErrorField(err))
				return err
			} else {
				if cmdTag, err := pool.Exec(context.Background(),
					`update car_state_proto set protodata=$1 where rs_info_id=$2`,
					binaryMessage, rsId); err != nil {
					log.Error("error updating car state", log.ErrorField(err))
				} else {
					updated += cmdTag.RowsAffected()
				}
			}
		}
		log.Info("updated car states",
			log.Uint32("eventId", eventId),
			log.Int64("updated", updated))
		return nil
	})
	if updateErr != nil {
		log.Error("error updating car states", log.ErrorField(updateErr))
	}
}
