package cli

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func applyRuntimeHarnessOverrides(effective effectiveHarnessConfig, flags globalFlags) effectiveHarnessConfig {
	cfg := effective.Config
	if cfg == nil {
		return effective
	}
	if quality := strings.TrimSpace(flags.Quality); quality != "" {
		cfg.Quality.Default = quality
	}
	entry, ok := cfg.Orchestra.Providers["codex"]
	if !ok || entry.ModelPolicy != config.ProviderModelPolicyQuality {
		return effective
	}
	profile := cfg.Quality.CodexOrchestraProfile()
	if effort := strings.TrimSpace(flags.Effort); effort != "" {
		profile.Effort = effort
	}
	cfg.Orchestra.Providers["codex"] = config.ApplyCodexProviderProfile(entry, profile)
	return effective
}

var (
	orchestraRunLoadConfig     = loadHarnessConfigForFlags
	orchestraRunBuildProviders = buildProviderConfigsForRuntime
	// orchestraRunBackendFactory routes the run pipeline backend through
	// SelectBackend (REQ-003) so it consumes the detected terminal rather than a
	// hardcoded subprocess backend. Kept as a var to preserve the test seam.
	orchestraRunBackendFactory  func(orchestra.OrchestraConfig) orchestra.ExecutionBackend = orchestra.SelectBackend
	orchestraRunExecutePipeline                                                            = orchestra.RunSubprocessPipeline
)
