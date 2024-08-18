package replay

import (
	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/cmd/replay/grpc"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/cmd/replay/util"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/cmd/replay/wamp"
)

func NewReplayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "replay",
		Short: "commands to replay an event from database",
	}

	cmd.PersistentFlags().IntVar(&util.Speed, "speed", 1,
		"Recording speed (0 means: go as fast as possible)")
	cmd.PersistentFlags().StringVar(&util.Addr,
		"addr",
		"localhost:8084",
		"gRPC server address")

	cmd.PersistentFlags().StringVarP(&util.Token,
		"token", "t", "", "authentication token")
	cmd.PersistentFlags().StringVar(&util.EventKey,
		"key", "", "event key to use for replay")
	cmd.PersistentFlags().BoolVar(&util.DoNotPersist,
		"do-not-persist", false, "do not persist data")
	cmd.PersistentFlags().StringVar(&util.FastForward,
		"fast-forward",
		"",
		"replay this duration with max speed")
	cmd.AddCommand(grpc.NewReplayGrpcCmd())
	cmd.AddCommand(wamp.NewReplayWampCmd())
	return cmd
}
