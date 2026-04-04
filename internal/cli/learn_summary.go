package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/learn"
)

// newLearnSummaryCmd returns the `auto learn summary` subcommand.
// --top defaults to 5.
func newLearnSummaryCmd() *cobra.Command {
	var topN int

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Print a learning summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			store, err := learn.NewStore(cwd)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}

			s, err := learn.GenerateSummary(store, topN)
			if err != nil {
				return fmt.Errorf("summary: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Total entries: %d\n\n", s.TotalEntries)

			fmt.Fprintln(out, "Type counts:")
			for t, count := range s.TypeCounts {
				fmt.Fprintf(out, "  %-20s %d\n", string(t), count)
			}

			if len(s.TopPatterns) > 0 {
				fmt.Fprintln(out, "\nTop patterns:")
				for _, p := range s.TopPatterns {
					fmt.Fprintf(out, "  [%dx] %s\n", p.ReuseCount, p.Pattern)
				}
			}

			if len(s.ImprovementAreas) > 0 {
				fmt.Fprintln(out, "\nImprovement areas:")
				for _, area := range s.ImprovementAreas {
					fmt.Fprintf(out, "  - %s\n", area)
				}
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&topN, "top", 5, "Number of top patterns to show")

	return cmd
}
