package util

import (
	"context"
	"database/sql"
	"errors"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	"connectrpc.com/connect"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

var (
	ErrMissingEventSelector     = errors.New("missing event selector")
	ErrInvalidEventSelector     = errors.New("invalid event selector")
	ErrMissingStartSelector     = errors.New("missing start selector")
	ErrInvalidStartSelector     = errors.New("invalid start selector")
	ErrUnsupportedStartSelector = errors.New("unsupported start selector")
)

// returns the event for the given selector
// if the event is not found, a connect error with code NotFound is returned
//
//nolint:whitespace // can't make both editor and linter happy
func ResolveEvent(
	ctx context.Context,
	r api.EventRepository,
	sel *commonv1.EventSelector,
) (*eventv1.Event, error) {
	var data *eventv1.Event
	var err error
	if sel.Arg == nil {
		return nil, ErrMissingEventSelector
	}
	switch sel.Arg.(type) {
	case *commonv1.EventSelector_Id:
		data, err = r.LoadByID(ctx, int(sel.GetId()))
	case *commonv1.EventSelector_Key:
		data, err = r.LoadByKey(ctx, sel.GetKey())
	default:
		err = ErrInvalidEventSelector
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, utils.ErrEventNotFound)
		}
		return nil, err
	}
	return data, nil
}
