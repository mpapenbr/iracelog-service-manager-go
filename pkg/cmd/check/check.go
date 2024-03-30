package check

import (
	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/spf13/cobra"
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
