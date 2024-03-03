package grpc

import (
	eventsv1grpc "buf.build/gen/go/mpapenbr/testrepo/grpc/go/testrepo/events/v1/eventsv1grpc"
	eventsv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/events/v1"
)

type eventsServer struct {
	X eventsv1.Event
	Y eventsv1grpc.EventServiceServer
}
