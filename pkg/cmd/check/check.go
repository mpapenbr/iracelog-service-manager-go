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

	cmd.AddCommand(NewCheckConvertCmd())
	cmd.AddCommand(NewCheckDataFetcherCmd())
	cmd.AddCommand(NewCheckSnapshotsCmd())
	cmd.AddCommand(NewCheckStateInoutlapCmd())
	cmd.AddCommand(NewDisplayLapsCmd())
	return cmd
}

var logLevel string

func parseLogLevel(l string, defaultVal log.Level) log.Level {
	level, err := log.ParseLevel(l)
	if err != nil {
		return defaultVal
	}
	return level
}
