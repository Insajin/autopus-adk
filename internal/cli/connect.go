package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

// @AX:NOTE [AUTO] @AX:REASON: hardcoded production server URL — overridable via --server flag
const defaultServerURL = "https://api.autopus.co"

func newConnectCmd() *cobra.Command {
	var (
		serverURL   string
		workspaceID string
		headless    bool
		timeout     time.Duration
	)

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect an AI provider via local OAuth flow",
		Long:  "Interactive wizard: server auth → workspace → OpenAI OAuth. Concretely: (1) Autopus server auth, (2) workspace selection, (3) OpenAI PKCE OAuth. For installed desktop operation, use the desktop app Connect action or `autopus-desktop-runtime connect`; `auto connect` is a retained compatibility shim.\n\nThis ADK surface delegates to the desktop-owned runtime helper.",
		RunE: func(cmd *cobra.Command, args []string) error {
			helperArgs := []string{"connect"}
			helperArgs = appendStringFlag(helperArgs, "server", serverURL, cmd.Flags().Changed("server"))
			helperArgs = appendStringFlag(helperArgs, "workspace", workspaceID, workspaceID != "")
			helperArgs = appendBoolFlag(helperArgs, "headless", headless)
			helperArgs = appendDurationFlag(helperArgs, "timeout", timeout, cmd.Flags().Changed("timeout"))
			return delegateRuntimeHelperStream(cmd, helperArgs)
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", defaultServerURL, "Autopus server URL")
	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Skip workspace selection and use this ID")
	cmd.Flags().BoolVar(&headless, "headless", false, "Non-interactive mode for agent-driven OAuth connection")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "Overall flow timeout for headless mode")
	cmd.AddCommand(newConnectStatusCmd())
	return cmd
}

// saveConnectConfig persists workspace/runtime state so desktop-owned commands
// and legacy worker compatibility surfaces read the same backend/workspace config.
func saveConnectConfig(wsID, serverURL string) error {
	cfg, err := setup.LoadWorkerConfig()
	if err != nil {
		// No existing config — create a new one.
		cfg = &setup.WorkerConfig{}
	}
	cfg.WorkspaceID = wsID
	if serverURL != "" {
		cfg.BackendURL = serverURL
	}
	return setup.SaveWorkerConfig(*cfg)
}
