package orchestra

import (
	"strings"
	"time"
)

func providerRunReceipt(response ProviderResponse, role string, attempt int, failed []FailedProvider) ProviderRunReceipt {
	provider := trimJudgeRole(response.Provider)
	receipt := ProviderRunReceipt{
		Provider: provider, Role: role, Attempt: attempt,
		Backend: response.ExecutedBackend, ExitCode: response.ExitCode,
		ModelFamily: response.ModelFamily,
		TimedOut:    response.TimedOut, Usable: responseUsable(response), Artifact: response.Receipt,
	}
	applyExecutionProvenance(&receipt, response.Execution)
	for _, entry := range failed {
		entryRole := entry.Role
		if entryRole == "" {
			entryRole = "participant"
		}
		entryAttempt := entry.Attempt
		if entryAttempt == 0 {
			entryAttempt = 1
		}
		if entry.Name == provider && entryRole == role && entryAttempt == attempt {
			receipt.FailureClass = entry.FailureClass
			break
		}
	}
	return receipt
}

// failedProviderRunReceipt projects an attempt that never produced a response
// row, keeping the launch provenance of the process that failed or timed out.
func failedProviderRunReceipt(failed FailedProvider, role string, attempt int) ProviderRunReceipt {
	receipt := ProviderRunReceipt{
		Provider: failed.Name, Role: role, Attempt: attempt,
		Backend:     firstNonempty(failed.ExecutedBackend, failed.CollectionMode),
		ModelFamily: failed.ModelFamily, ExitCode: failed.ExitCode,
		TimedOut: failed.TimedOut, Usable: false,
		FailureClass: failed.FailureClass, Artifact: failed.Receipt,
	}
	applyExecutionProvenance(&receipt, failed.Execution)
	return receipt
}

// applyExecutionProvenance copies launch evidence onto a receipt. Timestamps
// are RFC3339 strings so absent evidence omits the fields instead of emitting
// zero times.
func applyExecutionProvenance(receipt *ProviderRunReceipt, execution *ProviderExecution) {
	if execution == nil {
		return
	}
	receipt.Command = append([]string(nil), execution.Command...)
	receipt.Cwd = execution.Cwd
	receipt.PID = execution.PID
	receipt.SandboxMode = execution.SandboxMode
	receipt.StartedAt = receiptTimestamp(execution.StartedAt)
	receipt.EndedAt = receiptTimestamp(execution.EndedAt)
}

func receiptTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func responseUsable(response ProviderResponse) bool {
	return !response.TimedOut && !response.EmptyOutput && response.ExitCode == 0 && strings.TrimSpace(response.Output) != ""
}

func hasFailedReceipt(receipts []ProviderRunReceipt, provider, role string, attempt int) bool {
	for _, receipt := range receipts {
		if receipt.Provider == provider && receipt.Role == role && receipt.Attempt == attempt && !receipt.Usable {
			return true
		}
	}
	return false
}

func hasProviderReceipt(receipts []ProviderRunReceipt, provider string) bool {
	for _, receipt := range receipts {
		if receipt.Provider == provider {
			return true
		}
	}
	return false
}
