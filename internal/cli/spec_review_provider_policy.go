package cli

import (
	"time"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func specReviewProviderBackendName(primary orchestra.ExecutionBackend) string {
	if primary == nil {
		return "unknown"
	}
	return primary.Name()
}

// applySpecReviewExecutionTimeout copies the provider slice before applying an
// explicit CLI timeout. Zero means the flag was omitted, so provider defaults
// remain authoritative.
func applySpecReviewExecutionTimeout(providers []orchestra.ProviderConfig, requestedSeconds int) []orchestra.ProviderConfig {
	configured := append([]orchestra.ProviderConfig(nil), providers...)
	if requestedSeconds <= 0 {
		return configured
	}
	timeout := time.Duration(requestedSeconds) * time.Second
	for i := range configured {
		configured[i].ExecutionTimeout = timeout
	}
	return configured
}
