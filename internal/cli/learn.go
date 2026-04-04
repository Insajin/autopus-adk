// Package cli defines Cobra-based CLI commands.
package cli

import (
	"github.com/spf13/cobra"
)

// newLearnCmd creates the `auto learn` command group.
func newLearnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "learn",
		Short: "Manage pipeline learning entries",
	}

	cmd.AddCommand(newLearnQueryCmd())
	cmd.AddCommand(newLearnRecordCmd())
	cmd.AddCommand(newLearnPruneCmd())
	cmd.AddCommand(newLearnSummaryCmd())

	return cmd
}
