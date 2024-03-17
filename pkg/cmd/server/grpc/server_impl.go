package grpc

import (
	"context"
	"time"

	x "buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/events/v1/eventsv1connect"
	eventsv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/events/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/event"
)

func newServer(opts ...Option) *eventsServer {
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
	req *connect.Request[eventsv1.GetEventsRequest],
	stream *connect.ServerStream[eventsv1.GetEventsResponse],
) error {
	data, err := event.LoadAll(context.Background(), s.pool)
	if err != nil {
		return err
	}
	for i := range data {

		if err := stream.Send(
			&eventsv1.GetEventsResponse{Event: transform(data[i])}); err != nil {
			log.Error("Error sending event", log.ErrorField(err))
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *eventsServer) GetEvent(
	ctx context.Context, req *connect.Request[eventsv1.GetEventRequest],
) (*connect.Response[eventsv1.GetEventResponse], error) {
	log.Info("GetEvent called", log.Any("arg", req.Msg), log.Int32("id", req.Msg.GetId()))
	var data *model.DbEvent
	var err error
	switch req.Msg.Arg.(type) {
	case *eventsv1.GetEventRequest_Id:
		data, err = event.LoadById(ctx, s.pool, int(req.Msg.GetId()))
	case *eventsv1.GetEventRequest_Key:
		data, err = event.LoadByKey(context.Background(), s.pool, req.Msg.GetKey())
	}

	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&eventsv1.GetEventResponse{Event: transform(data)}), nil
}

func transform(db *model.DbEvent) *eventsv1.Event {
	return &eventsv1.Event{
		Id:  int32(db.ID),
		Key: db.Key,
	}
}
