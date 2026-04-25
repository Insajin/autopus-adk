package cli

import "github.com/spf13/cobra"

func newDesktopSessionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "session",
		Short: "Show desktop runtime session bootstrap as JSON",
		Long:  "Returns the desktop-owned runtime session bootstrap payload as machine-readable JSON. For installed desktop operation, prefer the desktop app session/bootstrap action or `autopus-desktop-runtime desktop session`; this ADK path is retained as a migration shim for compatibility.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return delegateRuntimeHelperStream(cmd, []string{"desktop", "session"})
		},
	}
}
