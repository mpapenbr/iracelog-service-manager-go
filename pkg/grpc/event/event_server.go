package event

import (
	"context"
	"time"

	x "buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/event/v1/eventv1connect"
	eventv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/event/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/event"
)

func NewServer(opts ...Option) *eventsServer {
	ret := &eventsServer{}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type Option func(*eventsServer)

func WithPool(p *pgxpool.Pool) Option {
	return func(srv *eventsServer) {
		srv.pool = p
	}
}

type eventsServer struct {
	x.UnimplementedEventServiceHandler

	pool *pgxpool.Pool
}

//nolint:whitespace // can't make both editor and linter happy
func (s *eventsServer) GetEvents(
	ctx context.Context,
	req *connect.Request[eventv1.GetEventsRequest],
	stream *connect.ServerStream[eventv1.GetEventsResponse],
) error {
	data, err := event.LoadAll(context.Background(), s.pool)
	if err != nil {
		return err
	}
	for i := range data {

		if err := stream.Send(
			&eventv1.GetEventsResponse{Event: transform(data[i])}); err != nil {
			log.Error("Error sending event", log.ErrorField(err))
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *eventsServer) GetEvent(
	ctx context.Context, req *connect.Request[eventv1.GetEventRequest],
) (*connect.Response[eventv1.GetEventResponse], error) {
	log.Debug("GetEvent called",
		log.Any("arg", req.Msg),
		log.Int32("id", req.Msg.EventSelector.GetId()))
	var data *model.DbEvent
	var err error
	switch req.Msg.EventSelector.Arg.(type) {
	case *eventv1.EventSelector_Id:
		data, err = event.LoadById(ctx, s.pool, int(req.Msg.EventSelector.GetId()))
	case *eventv1.EventSelector_Key:
		data, err = event.LoadByKey(context.Background(), s.pool,
			req.Msg.EventSelector.GetKey())
	}

	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&eventv1.GetEventResponse{Event: transform(data)}), nil
}

func transform(db *model.DbEvent) *eventv1.Event {
	return &eventv1.Event{
		Id:  uint32(db.ID),
		Key: db.Key,
	}
}
