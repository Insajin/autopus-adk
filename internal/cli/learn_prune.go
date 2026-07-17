package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/learn"
)

// newLearnPruneCmd returns the `auto learn prune` subcommand.
func newLearnPruneCmd() *cobra.Command {
	var days int
	var maxAge int

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

			pruneDays := days
			if cmd.Flags().Changed("max-age") {
				pruneDays = maxAge
			} else if !cmd.Flags().Changed("days") {
				return fmt.Errorf("either --days or --max-age is required")
			}

			removed, err := learn.Prune(store, pruneDays)
			if err != nil {
				return fmt.Errorf("prune: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed %d entries older than %d days.\n", removed, pruneDays)
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 0, "Remove entries older than this many days")
	cmd.Flags().IntVar(&maxAge, "max-age", 0, "Remove entries older than this many days (alternative to --days)")

	return cmd
}
