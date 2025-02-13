package repair

import (
	"github.com/spf13/cobra"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/cmd/repair/issue208"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/cmd/repair/issue244"
)

func NewRepairCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "commands to repair data",
	}
	cmd.AddCommand(issue208.NewIssue208Cmd())
	cmd.AddCommand(issue244.NewIssue244Cmd())
	return cmd
}
