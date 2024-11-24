package repair

import (
	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/cmd/repair/issue208"
)

func NewRepairCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "commands to repair data",
	}
	cmd.AddCommand(issue208.NewIssue208Cmd())
	return cmd
}
