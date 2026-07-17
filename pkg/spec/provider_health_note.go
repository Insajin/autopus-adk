package spec

import (
	"fmt"
	"strings"
	"time"
)

func timeoutProviderHealthNote(
	source string,
	budget time.Duration,
	elapsed time.Duration,
	collection string,
	partialOutput bool,
) string {
	return fmt.Sprintf(
		"timeout; source=%s; budget=%s; elapsed=%s; collection=%s; partial_output=%t",
		allowTimeoutSource(source),
		formatHealthDuration(budget),
		formatHealthDuration(elapsed),
		allowCollectionMode(collection),
		partialOutput,
	)
}

func allowTimeoutSource(source string) string {
	switch strings.TrimSpace(source) {
	case "spec_review_timeout",
		"provider_execution_timeout",
		"orchestra_timeout_seconds",
		"default_provider_execution_timeout":
		return strings.TrimSpace(source)
	default:
		return "unknown"
	}
}

func allowCollectionMode(collection string) string {
	switch strings.TrimSpace(collection) {
	case "hook", "poll", "file_ipc", "subprocess_stdout", "pane", "pane_receipt":
		return strings.TrimSpace(collection)
	default:
		return "unknown"
	}
}

func collectionModeFromBackend(backend string) string {
	switch strings.TrimSpace(backend) {
	case "subprocess":
		return "subprocess_stdout"
	case "pane":
		return "pane"
	default:
		return "unknown"
	}
}

func formatHealthDuration(duration time.Duration) string {
	if duration <= 0 {
		return "unknown"
	}
	return duration.String()
}

func hasPartialProviderOutput(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
