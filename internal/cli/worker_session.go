package cli

import "github.com/spf13/cobra"

func newWorkerSessionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "session",
		Short: "Show desktop session bootstrap as JSON (compatibility)",
		Long:  "Compatibility shim for the desktop session bootstrap payload. Prefer Autopus Desktop or `autopus-desktop-runtime desktop session`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return delegateRuntimeHelperStream(cmd, []string{"desktop", "session"})
		},
	}
}
