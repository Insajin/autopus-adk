package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/learn"
)

// newLearnPruneCmd returns the `auto learn prune` subcommand.
// --days is required.
func newLearnPruneCmd() *cobra.Command {
	var days int

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove learning entries older than N days",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			store, err := learn.NewStore(cwd)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}

			removed, err := learn.Prune(store, days)
			if err != nil {
				return fmt.Errorf("prune: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed %d entries older than %d days.\n", removed, days)
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 0, "Remove entries older than this many days")
	_ = cmd.MarkFlagRequired("days")

	return cmd
}
