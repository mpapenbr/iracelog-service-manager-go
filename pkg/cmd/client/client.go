//nolint:funlen // keeping by design
package client

import (
	"context"
	"net/http"
	"os"
	"strconv"

	livedataconnectv1 "buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/livedata/v1/livedatav1connect"
	eventv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/event/v1"
	livedatav1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/livedata/v1"
	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

var addr string

func NewLiveStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "receives live state data",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			liveStateData(args[0])
		},
	}
	return cmd
}

func liveStateData(eventArg string) {
	logger := log.DevLogger(
		os.Stderr,
		log.DebugLevel,
		log.WithCaller(true),
		log.AddCallerSkip(1))
	log.ResetDefault(logger)

	sel := ResolveEvent(eventArg)
	conn := livedataconnectv1.NewLiveDataServiceClient(http.DefaultClient, "http://localhost:8084")

	req := livedatav1.LiveRaceStateRequest{
		Event: sel,
	}
	r, err := conn.LiveRaceState(context.Background(), connect.NewRequest(&req))
	if err != nil {
		log.Error("could not get live data", log.ErrorField(err))
		return
	}

	for r.Receive() {
		resp := r.Msg()
		log.Debug("got state: ", log.Time("ts", resp.Timestamp.AsTime()))

	}
	log.Info("done")
}

func ResolveEvent(arg string) *eventv1.EventSelector {
	if id, err := strconv.Atoi(arg); err == nil {
		return &eventv1.EventSelector{Arg: &eventv1.EventSelector_Id{Id: int32(id)}}
	}
	return &eventv1.EventSelector{Arg: &eventv1.EventSelector_Key{Key: arg}}
}
