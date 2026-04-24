package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/connect"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

// @AX:NOTE [AUTO] @AX:REASON: hardcoded production server URL — overridable via --server flag
const (
	defaultServerURL = "https://api.autopus.co"
	// @AX:NOTE [AUTO] @AX:REASON: 5-minute timeout covers full 3-step wizard (server auth + workspace + OAuth)
	connectTimeout = 5 * time.Minute
)

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
		Long:  "Interactive wizard: server auth → workspace → OpenAI OAuth. Concretely: (1) Autopus server auth, (2) workspace selection, (3) OpenAI PKCE OAuth. Use `auto connect status` for deterministic local verify output.\n\nThis ADK surface delegates to the desktop-owned runtime helper.",
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

func stepServerAuth(ctx context.Context, serverURL string) (*connect.AuthResult, error) {
	cfg := connect.ServerAuthConfig{ServerURL: serverURL}
	return connect.AuthenticateServer(ctx, cfg, nil)
}

func stepWorkspaceSelect(ctx context.Context, client *connect.Client, preselected string) (id, name string, err error) {
	workspaces, err := client.ListWorkspaces(ctx)
	if err != nil {
		return "", "", err
	}
	if len(workspaces) == 0 {
		// @AX:NOTE [AUTO] @AX:REASON: hardcoded app URL in error message — update if frontend domain changes
		return "", "", fmt.Errorf("no workspaces found — create one at https://app.autopus.co first")
	}

	// If --workspace flag provided, find the matching workspace.
	if preselected != "" {
		for _, ws := range workspaces {
			if ws.ID == preselected {
				return ws.ID, ws.Name, nil
			}
		}
		return "", "", fmt.Errorf("workspace %q not found", preselected)
	}

	// Single workspace: auto-select without prompting.
	if len(workspaces) == 1 {
		return workspaces[0].ID, workspaces[0].Name, nil
	}

	return promptWorkspaceSelect(workspaces)
}

func promptWorkspaceSelect(workspaces []connect.Workspace) (id, name string, err error) {
	// Guard against non-TTY hang: huh forms block indefinitely without a terminal.
	if !isStdinTTY() {
		return workspaces[0].ID, workspaces[0].Name, nil
	}

	options := make([]huh.Option[string], len(workspaces))
	for i, ws := range workspaces {
		label := ws.Name
		if ws.Description != "" {
			label = fmt.Sprintf("%s — %s", ws.Name, ws.Description)
		}
		options[i] = huh.NewOption(label, ws.ID)
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a workspace").
				Options(options...).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return "", "", fmt.Errorf("workspace selection cancelled: %w", err)
	}

	for _, ws := range workspaces {
		if ws.ID == selected {
			return ws.ID, ws.Name, nil
		}
	}
	return selected, selected, nil
}

func stepOAuth(ctx context.Context) (*connect.OAuthResult, error) {
	cfg := connect.OAuthConfig{ClientID: connect.DefaultClientID()}
	return connect.WaitForCallback(ctx, cfg)
}

func stepSubmitToken(ctx context.Context, client *connect.Client, wsID string, oauth *connect.OAuthResult) error {
	req := connect.SubmitTokenRequest{
		ProviderToken: oauth.AccessToken,
		RefreshToken:  oauth.RefreshToken,
		WorkspaceID:   wsID,
		Provider:      "openai",
	}
	return client.SubmitToken(ctx, req)
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
