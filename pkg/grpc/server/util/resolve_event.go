package util

import (
	"context"
	"errors"

	commonv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/event/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

// returns the event for the given selector
// if the event is not found, a connect error with code NotFound is returned
//
//nolint:whitespace // can't make both editor and linter happy
func ResolveEvent(
	ctx context.Context,
	conn repository.Querier,
	sel *commonv1.EventSelector,
) (*eventv1.Event, error) {
	var data *eventv1.Event
	var err error
	switch sel.Arg.(type) {
	case *commonv1.EventSelector_Id:
		data, err = event.LoadById(ctx, conn, int(sel.GetId()))
	case *commonv1.EventSelector_Key:
		data, err = event.LoadByKey(ctx, conn, sel.GetKey())

	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, utils.ErrEventNotFound)
		}
		return nil, err
	}
	return data, nil
}
