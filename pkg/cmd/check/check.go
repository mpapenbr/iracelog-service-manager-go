package check

import (
	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

func NewCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "commands to check",
	}

	cmd.AddCommand(NewCheckDataFetcherCmd())
	cmd.AddCommand(NewCheckCarStatesCmd())
	cmd.AddCommand(NewCheckSnapshotsBobCmd())
	cmd.AddCommand(NewCheckSnapshotsBobLegacyCmd())
	cmd.AddCommand(NewCheckStateInoutlapCmd())
	cmd.AddCommand(NewDisplayLapsCmd())

	return cmd
}

var logLevel string

var startTS float64

func parseLogLevel(l string, defaultVal log.Level) log.Level {
	level, err := log.ParseLevel(l)
	if err != nil {
		return defaultVal
	}
	return level
}
