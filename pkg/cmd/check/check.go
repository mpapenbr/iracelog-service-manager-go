package check

import (
	"github.com/spf13/cobra"
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

var startTS float64
