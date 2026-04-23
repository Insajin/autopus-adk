package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

type ensureRunner func(context.Context, string, string) (*setup.EnsureResult, error)

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
			"Compatibility shim: prefer `auto desktop ensure` for the canonical desktop-owned readiness contract.",
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

func runEnsureCommand(cmd *cobra.Command, workspaceID, backendURL string, runner ensureRunner) error {
	if workspaceID == "" {
		return fmt.Errorf("--workspace is required")
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
	defer cancel()

	result, err := runner(ctx, backendURL, workspaceID)
	if err != nil && result == nil {
		errResult := &setup.EnsureResult{
			Action: "error",
			Data:   map[string]string{"message": err.Error()},
		}
		writeJSON(cmd, errResult)
		os.Exit(1)
		return nil
	}

	if result == nil {
		result = &setup.EnsureResult{
			Action: "error",
			Data:   map[string]string{"message": "ensure returned no result"},
		}
	}

	writeJSON(cmd, result)

	switch result.Action {
	case "ready", "starting_daemon":
		os.Exit(0)
	case "login_required":
		os.Exit(2)
	case "error":
		os.Exit(1)
	default:
		os.Exit(0)
	}
	return nil
}

// writeJSON encodes result as indented JSON to the command's stdout.
func writeJSON(cmd *cobra.Command, v any) {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "json encode error: %v\n", err)
	}
}
