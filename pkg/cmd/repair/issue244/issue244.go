package issue244

// copies session num to rs_info
// see issue #244

import (
	"context"
	"time"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
	"github.com/stephenafamo/bob"
	"google.golang.org/protobuf/proto"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/postgres"
	eventrepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewIssue244Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue244",
		Short: "copies session num from state message to rs_info entry",

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			copySessionNums(cmd.Context())
		},
	}
	return cmd
}

func copySessionNums(ctx context.Context) {
	// wait for database
	timeout, err := time.ParseDuration(config.WaitForServices)
	if err != nil {
		log.Warn("Invalid duration value. Setting default 60s", log.ErrorField(err))
		timeout = 60 * time.Second
	}

	postgresAddr := utils.ExtractFromDBURL(config.DB)
	if err = utils.WaitForTCP(postgresAddr, timeout); err != nil {
		log.Fatal("database  not ready", log.ErrorField(err))
	}
	pool := postgres.InitWithURL(config.DB)
	defer pool.Close()
	db := bob.NewDB(stdlib.OpenDBFromPool(pool))
	eRepo := eventrepo.NewEventRepository(db)
	if events, err := eRepo.LoadAll(ctx, nil); err == nil {
		for _, event := range events {
			doMigrateRaceStateSessionNums(pool, event.Id)
		}
	} else {
		log.Error("error loading events", log.ErrorField(err))
	}
}

//nolint:funlen // long function
func doMigrateRaceStateSessionNums(pool *pgxpool.Pool, eventID uint32) {
	// we update only by race_state_proto data
	// car_state_proto entries prior to race session are mostly associcated
	// to one dummy entry
	// therefore we don't create new rs_info entries for those.
	// we keep them at session_num=0
	updateErr := pgx.BeginFunc(context.Background(), pool, func(tx pgx.Tx) error {
		rows, err := pool.Query(context.Background(), `
select
rsp.protodata, ri.record_stamp,ri.session_time,ri.id
from race_state_proto rsp
join rs_info ri on ri.id=rsp.rs_info_id

where
ri.event_id=$1
order by ri.id asc
	`,
			eventID,
		)
		if err != nil {
			log.Error("error fetching car states", log.ErrorField(err))
			return err
		}
		defer rows.Close()
		var updated int64 = 0
		for rows.Next() {
			var rsProtodata []byte

			var recordStamp time.Time
			var sessionTime float64
			var rsID int
			err = rows.Scan(&rsProtodata, &recordStamp, &sessionTime, &rsID)
			if err != nil {
				log.Error("error scanning car states", log.ErrorField(err))
				return err
			}

			racestate := &racestatev1.PublishStateRequest{}
			if err := proto.Unmarshal(rsProtodata, racestate); err != nil {
				log.Error("error unmarshalling racestate", log.ErrorField(err))
				return err
			}
			log.Debug("race state", log.Float64("sessionTime", sessionTime),
				log.Uint32("sessionNum (rs)", racestate.Session.SessionNum),
				log.Int("rsId", rsID),
			)

			if cmdTag, err := pool.Exec(context.Background(),
				`update rs_info set session_num=$1 where id=$2`,
				racestate.Session.SessionNum, rsID); err != nil {
				log.Error("error updating car state", log.ErrorField(err))
			} else {
				updated += cmdTag.RowsAffected()
			}
		}
		log.Info("updated car states",
			log.Uint32("eventId", eventID),
			log.Int64("updated", updated))
		return nil
	})
	if updateErr != nil {
		log.Error("error updating car states", log.ErrorField(updateErr))
	}
}
