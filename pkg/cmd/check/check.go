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
	cmd.PersistentFlags().StringVar(&logLevel,
		"log-level",
		"info",
		"controls the log level (debug, info, warn, error, fatal)")

	cmd.AddCommand(NewCheckConvertCmd())
	cmd.AddCommand(NewCheckDataFetcherCmd())
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
