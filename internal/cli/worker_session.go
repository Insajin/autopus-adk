package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

func newWorkerSessionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "session",
		Short: "Show desktop session bootstrap as JSON",
		Long:  "Returns the local desktop session bootstrap payload as machine-readable JSON.",
		RunE: func(cmd *cobra.Command, args []string) error {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(setup.LoadDesktopSession())
		},
	}
}
