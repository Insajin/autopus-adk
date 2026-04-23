package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	workerSetup "github.com/insajin/autopus-adk/pkg/worker/setup"
)

func newDesktopStatusCmd() *cobra.Command {
	var jsonOutput bool
	var format string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show desktop runtime readiness",
		Long:  "Desktop-owned runtime readiness surface. Use `--json` for the canonical machine-readable contract. Packaged runtime source/build ownership now lives in `autopus-desktop/runtime-helper`; this ADK path is retained for compatibility and harness verification.",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, err := resolveJSONMode(jsonOutput, format)
			if err != nil {
				return err
			}
			helperArgs := []string{"desktop", "status"}
			helperArgs = appendBoolFlag(helperArgs, "json", jsonMode)
			helperArgs = appendStringFlag(helperArgs, "format", format, cmd.Flags().Changed("format"))
			if jsonMode {
				return delegateRuntimeHelperJSON(cmd, helperArgs)
			}
			return delegateRuntimeHelperStream(cmd, helperArgs)
		},
	}

	addJSONFlags(cmd, &jsonOutput, &format)
	return cmd
}

func buildDesktopStatusWarnings(status workerSetup.WorkerStatus) []jsonMessage {
	warnings := make([]jsonMessage, 0)
	if !status.Configured {
		warnings = append(warnings, jsonMessage{
			Code:    "desktop_not_configured",
			Message: "Desktop runtime configuration is missing.",
		})
	}
	if !status.AuthValid || status.AuthType != "jwt" {
		warnings = append(warnings, jsonMessage{
			Code:    "desktop_auth_invalid",
			Message: "Desktop runtime requires valid JWT credentials.",
		})
	}
	if !status.SecureStorageReady {
		warnings = append(warnings, jsonMessage{
			Code:    "desktop_secure_storage_unavailable",
			Message: "Desktop runtime secure credential storage is unavailable.",
		})
	}
	if !status.DesktopSessionReady {
		warnings = append(warnings, jsonMessage{
			Code:    "desktop_session_not_ready",
			Message: "Desktop runtime session is not ready.",
		})
	}
	return warnings
}

func printDesktopStatus(out io.Writer, payload workerSetup.WorkerStatus) {
	fmt.Fprintf(out, "Ready: %v\n", payload.DesktopSessionReady)
	fmt.Fprintf(out, "Configured: %v\n", payload.Configured)
	fmt.Fprintf(out, "Auth valid: %v (%s)\n", payload.AuthValid, payload.AuthType)
	fmt.Fprintf(out, "Workspace: %s\n", defaultConnectValue(payload.WorkspaceID))
	fmt.Fprintf(out, "Backend: %s\n", defaultConnectValue(payload.BackendURL))
	fmt.Fprintf(out, "Credential backend: %s\n", defaultConnectValue(payload.CredentialBackend))
	fmt.Fprintf(out, "Secure storage ready: %v\n", payload.SecureStorageReady)
	fmt.Fprintf(out, "Desktop session ready: %v\n", payload.DesktopSessionReady)
	fmt.Fprintf(out, "Next: %s\n", nextDesktopStatusAction(payload))
}

func nextDesktopStatusAction(status workerSetup.WorkerStatus) string {
	switch {
	case !status.Configured:
		return "Run `auto connect` or complete the desktop auth flow to persist runtime state."
	case !status.AuthValid || status.AuthType != "jwt":
		return "Run `auto connect` or complete the desktop auth flow to refresh JWT credentials."
	case status.WorkspaceID == "":
		return "Re-run `auto connect --workspace <id>` to persist the workspace selection."
	case status.BackendURL == "":
		return "Re-run `auto connect --server <url>` to persist the backend URL."
	case !status.SecureStorageReady:
		return "Re-run the desktop auth flow on a secure credential backend."
	case !status.DesktopSessionReady:
		return "Desktop runtime state is still incomplete; retry after auth/config recovery."
	default:
		return "Desktop runtime is ready."
	}
}
