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
		Short: "Ensure worker is ready (agent-native, JSON output)",
		Long: `Checks worker state and takes action to bring it to ready.
All output is JSON. Exit codes: 0=ready, 1=error, 2=login_required.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if workspaceID == "" {
				return fmt.Errorf("--workspace is required")
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			result, err := setup.EnsureWorker(ctx, backendURL, workspaceID)
			if err != nil && result == nil {
				// Fatal error before any result was produced.
				errResult := &setup.EnsureResult{
					Action: "error",
					Data:   map[string]string{"message": err.Error()},
				}
				writeJSON(cmd, errResult)
				os.Exit(1)
				return nil
			}

			if result != nil {
				writeJSON(cmd, result)
			}

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
		},
	}

	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Workspace ID (required)")
	cmd.Flags().StringVar(&backendURL, "backend", "https://api.autopus.co", "Backend API URL")
	_ = cmd.MarkFlagRequired("workspace")
	return cmd
}

// writeJSON encodes result as indented JSON to the command's stdout.
func writeJSON(cmd *cobra.Command, v any) {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "json encode error: %v\n", err)
	}
}
