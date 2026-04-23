package cli

import "github.com/spf13/cobra"

func newDesktopEnsureCmd() *cobra.Command {
	var workspaceID string
	var backendURL string

	cmd := &cobra.Command{
		Use:   "ensure",
		Short: "Ensure desktop runtime readiness (agent-native, JSON output)",
		Long: "Checks desktop runtime bootstrap state and brings it to ready without starting the legacy local-host daemon.\n" +
			"All output is JSON. Exit codes: 0=ready, 1=error, 2=login_required.\n\n" +
			"Packaged runtime source/build ownership now lives in `autopus-desktop/runtime-helper`; this ADK path is retained for compatibility and harness verification.",
		RunE: func(cmd *cobra.Command, args []string) error {
			helperArgs := []string{"desktop", "ensure", "--workspace", workspaceID}
			helperArgs = appendStringFlag(helperArgs, "backend", backendURL, cmd.Flags().Changed("backend"))
			return delegateRuntimeHelperStream(cmd, helperArgs)
		},
	}

	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Workspace ID (required)")
	cmd.Flags().StringVar(&backendURL, "backend", defaultServerURL, "Backend API URL")
	cmd.SilenceUsage = true
	_ = cmd.MarkFlagRequired("workspace")
	return cmd
}
