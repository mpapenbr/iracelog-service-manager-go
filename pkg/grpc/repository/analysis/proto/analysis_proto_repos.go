//nolint:whitespace // can't make both editor and linter happy
package proto

import (
	"context"
	"time"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	"google.golang.org/protobuf/proto"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository"
)

func Upsert(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
	analysis *analysisv1.Analysis,
) error {
	binaryMessage, err := proto.Marshal(analysis)
	if err != nil {
		return err
	}
	_, err = conn.Exec(ctx, `
	insert into analysis_proto (
		event_id, record_stamp, protodata
	) values ($1,$2,$3)
	on conflict (event_id) do update set record_stamp=$2, protodata=$3
		`,
		eventID, time.Now(), binaryMessage,
	)

	return err
}

func LoadByEventID(
	ctx context.Context,
	conn repository.Querier,
	eventID int,
) (*analysisv1.Analysis, error) {
	row := conn.QueryRow(ctx, `
	select protodata from analysis_proto where event_id=$1
	`, eventID)

	var binaryMessage []byte
	if err := row.Scan(&binaryMessage); err != nil {
		return nil, err
	}

	analysis := &analysisv1.Analysis{}
	if err := proto.Unmarshal(binaryMessage, analysis); err != nil {
		return nil, err
	}

	return analysis, nil
}

//nolint:lll // readability
func LoadByEventKey(
	ctx context.Context,
	conn repository.Querier,
	eventKey string,
) (*analysisv1.Analysis, error) {
	row := conn.QueryRow(ctx, `
	select protodata from analysis_proto where event_id=(select id from event where event_key=$1)
	`, eventKey)

	var binaryMessage []byte
	if err := row.Scan(&binaryMessage); err != nil {
		return nil, err
	}

	analysis := &analysisv1.Analysis{}
	if err := proto.Unmarshal(binaryMessage, analysis); err != nil {
		return nil, err
	}

	return analysis, nil
}

// deletes an entry from the database, returns number of rows deleted.
//
//nolint:lll // readability
func DeleteByEventID(ctx context.Context, conn repository.Querier, eventID int) (int, error) {
	cmdTag, err := conn.Exec(ctx,
		"delete from analysis_proto where event_id=$1", eventID)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}
