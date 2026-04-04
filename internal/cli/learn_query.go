package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/learn"
)

// newLearnQueryCmd returns the `auto learn query` subcommand.
// Accepts --files, --packages, --keywords as string slices.
func newLearnQueryCmd() *cobra.Command {
	var files, packages, keywords []string

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query relevant learning entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			store, err := learn.NewStore(cwd)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}

			q := learn.RelevanceQuery{
				Files:    files,
				Packages: packages,
				Keywords: keywords,
			}

			entries, err := learn.QueryRelevant(store, q, 1.0)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}

			out := cmd.OutOrStdout()
			if len(entries) == 0 {
				fmt.Fprintln(out, "No matching entries found.")
				return nil
			}

			fmt.Fprintf(out, "%-10s %-16s %-8s %s\n", "ID", "Type", "Score", "Pattern")
			fmt.Fprintf(out, "%-10s %-16s %-8s %s\n", "----------", "----------------", "--------", "-------")
			for _, e := range entries {
				fmt.Fprintf(out, "%-10s %-16s %-8.2f %s\n", e.ID, string(e.Type), 1.0, e.Pattern)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&files, "files", nil, "File paths to match against")
	cmd.Flags().StringSliceVar(&packages, "packages", nil, "Package names to match against")
	cmd.Flags().StringSliceVar(&keywords, "keywords", nil, "Keywords to match against")

	return cmd
}
