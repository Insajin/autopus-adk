package cli

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

var specReviewSubprocessBackendFactory = orchestra.NewSubprocessBackendImpl

// selectSpecReviewProviderBackend keeps pane execution as the review default,
// except for Codex's headless `exec` contract. That command emits its final
// answer through subprocess-only capture and must be selected before Execute.
func selectSpecReviewProviderBackend(
	primary orchestra.ExecutionBackend,
	provider orchestra.ProviderConfig,
) orchestra.ExecutionBackend {
	if !shouldPreselectSpecReviewSubprocess(primary, provider) {
		return primary
	}
	return specReviewSubprocessBackendFactory()
}

func specReviewProviderBackendName(primary orchestra.ExecutionBackend, provider orchestra.ProviderConfig) string {
	if shouldPreselectSpecReviewSubprocess(primary, provider) {
		return "subprocess"
	}
	if primary == nil {
		return "unknown"
	}
	return primary.Name()
}

func shouldPreselectSpecReviewSubprocess(
	primary orchestra.ExecutionBackend,
	provider orchestra.ProviderConfig,
) bool {
	if primary == nil || primary.Name() != "pane" || !isSpecReviewCodexProvider(provider) {
		return false
	}
	for _, arg := range provider.Args {
		if strings.TrimSpace(arg) == "" {
			continue
		}
		return arg == "exec"
	}
	return false
}

func isSpecReviewCodexProvider(provider orchestra.ProviderConfig) bool {
	name := strings.TrimSpace(provider.Name)
	binary := filepath.Base(strings.TrimSpace(provider.Binary))
	return strings.EqualFold(name, "codex") || strings.EqualFold(binary, "codex")
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
