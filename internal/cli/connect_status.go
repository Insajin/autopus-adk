package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	workerSetup "github.com/insajin/autopus-adk/pkg/worker/setup"
)

type connectStatusPayload struct {
	Ready      bool                         `json:"ready"`
	NextAction string                       `json:"next_action,omitempty"`
	Worker     workerSetup.WorkerStatus     `json:"worker"`
	Providers  []workerSetup.ProviderStatus `json:"providers"`
}

func newConnectStatusCmd() *cobra.Command {
	var jsonOutput bool
	var format string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show connect readiness and remaining manual steps",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, err := resolveJSONMode(jsonOutput, format)
			if err != nil {
				return err
			}

			payload := collectConnectStatusPayload()
			warnings := buildConnectStatusWarnings(payload)
			status := jsonStatusOK
			if len(warnings) > 0 {
				status = jsonStatusWarn
			}

			if jsonMode {
				return writeJSONResult(cmd, status, payload, warnings, nil)
			}

			printConnectStatus(cmd.OutOrStdout(), payload)
			return nil
		},
	}

	addJSONFlags(cmd, &jsonOutput, &format)
	return cmd
}

func collectConnectStatusPayload() connectStatusPayload {
	workerStatus := workerSetup.CollectStatus()
	ready := workerStatus.Configured &&
		workerStatus.AuthValid &&
		workerStatus.AuthType == "jwt" &&
		workerStatus.WorkspaceID != "" &&
		workerStatus.BackendURL != ""

	return connectStatusPayload{
		Ready:      ready,
		NextAction: nextConnectAction(workerStatus, ready),
		Worker:     workerStatus,
		Providers:  workerSetup.DetectProviders(),
	}
}

func nextConnectAction(status workerSetup.WorkerStatus, ready bool) string {
	switch {
	case ready:
		return "Use `auto desktop status --json` or start the worker when you need platform-connected execution."
	case !status.Configured:
		return "Run `auto connect` to authenticate with the server and select a workspace."
	case !status.AuthValid:
		return "Re-run `auto connect` to refresh Autopus server credentials."
	case status.WorkspaceID == "":
		return "Re-run `auto connect --workspace <id>` to save the workspace selection."
	case status.BackendURL == "":
		return "Re-run `auto connect --server <url>` to persist the backend URL."
	default:
		return "Use `auto desktop status --json` to inspect the desktop runtime readiness state."
	}
}

func buildConnectStatusWarnings(payload connectStatusPayload) []jsonMessage {
	warnings := make([]jsonMessage, 0)
	if !payload.Worker.Configured {
		warnings = append(warnings, jsonMessage{
			Code:    "connect_not_configured",
			Message: "Connect flow has not saved worker configuration yet.",
		})
	}
	if !payload.Worker.AuthValid {
		warnings = append(warnings, jsonMessage{
			Code:    "connect_auth_invalid",
			Message: "Autopus server credentials are missing or expired.",
		})
	}
	if payload.Worker.AuthType != "" && payload.Worker.AuthType != "jwt" {
		warnings = append(warnings, jsonMessage{
			Code:    "connect_auth_type",
			Message: "Desktop/worker verify flow expects JWT-based connect credentials.",
		})
	}
	return warnings
}

func printConnectStatus(out io.Writer, payload connectStatusPayload) {
	fmt.Fprintf(out, "Ready: %v\n", payload.Ready)
	fmt.Fprintf(out, "Configured: %v\n", payload.Worker.Configured)
	fmt.Fprintf(out, "Auth valid: %v (%s)\n", payload.Worker.AuthValid, payload.Worker.AuthType)
	fmt.Fprintf(out, "Workspace: %s\n", defaultConnectValue(payload.Worker.WorkspaceID))
	fmt.Fprintf(out, "Backend: %s\n", defaultConnectValue(payload.Worker.BackendURL))
	fmt.Fprintf(out, "Credential backend: %s\n", defaultConnectValue(payload.Worker.CredentialBackend))
	fmt.Fprintf(out, "Next: %s\n", payload.NextAction)

	if len(payload.Providers) == 0 {
		return
	}

	fmt.Fprintln(out, "Providers:")
	for _, provider := range payload.Providers {
		state := "missing"
		if provider.Installed {
			state = "installed"
		}
		if provider.Version != "" {
			fmt.Fprintf(out, "- %s: %s (%s)\n", provider.Name, state, provider.Version)
			continue
		}
		fmt.Fprintf(out, "- %s: %s\n", provider.Name, state)
	}
}

func defaultConnectValue(value string) string {
	if value == "" {
		return "(not set)"
	}
	return value
}
