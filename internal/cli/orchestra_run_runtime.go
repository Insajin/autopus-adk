package cli

import (
	"context"
	"fmt"
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

func providerConfigNames(providers []orchestra.ProviderConfig) []string {
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		names = append(names, provider.Name)
	}
	return names
}

func executeOrchestraRunStrategy(
	ctx context.Context,
	strategy orchestra.Strategy,
	cfg orchestra.OrchestraConfig,
	pipelineCfg orchestra.SubprocessPipelineConfig,
) (*orchestra.OrchestraResult, error) {
	switch strategy {
	case orchestra.StrategyConsensus:
		return runOrchestraExecute(ctx, cfg)
	case orchestra.StrategyDebate:
		result, err := orchestraRunExecutePipeline(ctx, pipelineCfg)
		if result != nil {
			result.Strategy = orchestra.StrategyDebate
			result = orchestra.FinalizeOrchestrationResult(result, cfg)
		}
		return result, err
	default:
		return nil, fmt.Errorf("unsupported orchestra run strategy %q", strategy)
	}
}
