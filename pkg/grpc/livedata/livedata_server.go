package livedata

import (
	"context"

	x "buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/livedata/v1/livedatav1connect"
	livedatav1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/livedata/v1"
	"connectrpc.com/connect"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

func NewServer(opts ...Option) *liveDataServer {
	ret := &liveDataServer{}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type Option func(*liveDataServer)

func WithEventLookup(lookup *utils.EventLookup) Option {
	return func(srv *liveDataServer) {
		srv.lookup = lookup
	}
}

type liveDataServer struct {
	x.UnimplementedLiveDataServiceHandler

	lookup *utils.EventLookup
}

//nolint:whitespace // can't make both editor and linter happy
func (s *liveDataServer) LiveRaceState(
	ctx context.Context,
	req *connect.Request[livedatav1.LiveRaceStateRequest],
	stream *connect.ServerStream[livedatav1.LiveRaceStateResponse],
) error {
	// get the epd
	epd, err := s.lookup.GetEvent(req.Msg.Event)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	log.Debug("Sending live race states",
		log.String("event", epd.Event.Key),
	)

	return nil
}
