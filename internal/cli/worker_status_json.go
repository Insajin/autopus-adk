package cli

import workerSetup "github.com/insajin/autopus-adk/pkg/worker/setup"

func buildWorkerStatusWarnings(status workerSetup.WorkerStatus) []jsonMessage {
	warnings := make([]jsonMessage, 0)
	if !status.Configured {
		warnings = append(warnings, jsonMessage{
			Code:    "worker_not_configured",
			Message: "Worker configuration is missing.",
		})
	}
	if !status.AuthValid {
		warnings = append(warnings, jsonMessage{
			Code:    "worker_auth_invalid",
			Message: "Worker credentials are missing or invalid.",
		})
	}
	if !status.DaemonRunning {
		warnings = append(warnings, jsonMessage{
			Code:    "worker_daemon_stopped",
			Message: "Worker daemon is not running.",
		})
	}
	return warnings
}
