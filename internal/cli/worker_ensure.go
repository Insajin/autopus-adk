package cli

import "github.com/spf13/cobra"

// newWorkerEnsureCmd returns the `auto worker ensure` cobra command.
// This is an agent-native command — all output is JSON.
//
// Exit codes:
//
//	0 = ready or starting_daemon (daemon started = success)
//	1 = error
//	2 = login_required (human interaction needed)
func newWorkerEnsureCmd() *cobra.Command {
	var workspaceID string
	var backendURL string

	cmd := &cobra.Command{
		Use:   "ensure",
		Short: "Ensure legacy worker is ready (compatibility JSON output)",
		Long: "Checks worker state and takes action to bring it to ready.\n" +
			"All output is JSON. Exit codes: 0=ready, 1=error, 2=login_required.\n\n" +
			"Compatibility shim: for installed desktop operation, prefer the desktop app readiness flow or `autopus-desktop-runtime desktop ensure` for the canonical desktop-owned readiness contract.",
		RunE: func(cmd *cobra.Command, args []string) error {
			helperArgs := []string{"worker", "ensure", "--workspace", workspaceID}
			helperArgs = appendStringFlag(helperArgs, "backend", backendURL, cmd.Flags().Changed("backend"))
			return delegateRuntimeHelperStream(cmd, helperArgs)
		},
	}

	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Workspace ID (required)")
	cmd.Flags().StringVar(&backendURL, "backend", "https://api.autopus.co", "Backend API URL")
	_ = cmd.MarkFlagRequired("workspace")
	return cmd
}
