package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

type noopExecutionBackend struct{}

func (noopExecutionBackend) Execute(context.Context, orchestra.ProviderRequest) (*orchestra.ProviderResponse, error) {
	return &orchestra.ProviderResponse{}, nil
}

func (noopExecutionBackend) Name() string {
	return "noop"
}

func successfulDebateRunResult(provider string) *orchestra.OrchestraResult {
	return &orchestra.OrchestraResult{
		Strategy:    orchestra.StrategyDebate,
		Responses:   []orchestra.ProviderResponse{{Provider: provider, Output: "usable result"}},
		Merged:      "ok",
		Summary:     "done",
		JudgeStatus: orchestra.JudgePassed,
	}
}

func TestRunSubprocessPipeline_UsesConfigTimeoutWhenFlagUnchanged(t *testing.T) {
	origLoadConfig := orchestraRunLoadConfig
	origBuildProviders := orchestraRunBuildProviders
	origBackendFactory := orchestraRunBackendFactory
	origExecutePipeline := orchestraRunExecutePipeline
	t.Cleanup(func() {
		orchestraRunLoadConfig = origLoadConfig
		orchestraRunBuildProviders = origBuildProviders
		orchestraRunBackendFactory = origBackendFactory
		orchestraRunExecutePipeline = origExecutePipeline
	})

	orchestraRunLoadConfig = func(globalFlags) (*config.HarnessConfig, error) {
		return &config.HarnessConfig{
			Orchestra: config.OrchestraConf{
				TimeoutSeconds: 240,
				Providers: map[string]config.ProviderEntry{
					"claude": {Binary: "claude"},
				},
			},
		}, nil
	}
	orchestraRunBuildProviders = buildProviderConfigsForRuntime
	orchestraRunBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return noopExecutionBackend{} }

	var captured orchestra.SubprocessPipelineConfig
	orchestraRunExecutePipeline = func(_ context.Context, cfg orchestra.SubprocessPipelineConfig) (*orchestra.OrchestraResult, error) {
		captured = cfg
		return successfulDebateRunResult(cfg.Providers[0].Name), nil
	}

	err := runSubprocessPipeline(context.Background(), "topic", "debate", []string{"claude"}, "standard", 120, false, "", false, false)
	require.NoError(t, err)
	assert.Equal(t, 240, captured.TimeoutSeconds)
}

func TestRunSubprocessPipeline_CLITimeoutOverridesConfig(t *testing.T) {
	origLoadConfig := orchestraRunLoadConfig
	origBuildProviders := orchestraRunBuildProviders
	origBackendFactory := orchestraRunBackendFactory
	origExecutePipeline := orchestraRunExecutePipeline
	t.Cleanup(func() {
		orchestraRunLoadConfig = origLoadConfig
		orchestraRunBuildProviders = origBuildProviders
		orchestraRunBackendFactory = origBackendFactory
		orchestraRunExecutePipeline = origExecutePipeline
	})

	orchestraRunLoadConfig = func(globalFlags) (*config.HarnessConfig, error) {
		return &config.HarnessConfig{
			Orchestra: config.OrchestraConf{
				TimeoutSeconds: 240,
				Providers: map[string]config.ProviderEntry{
					"claude": {Binary: "claude"},
				},
			},
		}, nil
	}
	orchestraRunBuildProviders = buildProviderConfigsForRuntime
	orchestraRunBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return noopExecutionBackend{} }

	var captured orchestra.SubprocessPipelineConfig
	orchestraRunExecutePipeline = func(_ context.Context, cfg orchestra.SubprocessPipelineConfig) (*orchestra.OrchestraResult, error) {
		captured = cfg
		return successfulDebateRunResult(cfg.Providers[0].Name), nil
	}

	err := runSubprocessPipeline(context.Background(), "topic", "debate", []string{"claude"}, "standard", 90, true, "", false, false)
	require.NoError(t, err)
	assert.Equal(t, 90, captured.TimeoutSeconds)
}

func TestRunSubprocessPipeline_ExplicitProvidersDoNotUseExcludedConfigJudge(t *testing.T) {
	origLoadConfig := orchestraRunLoadConfig
	origBuildProviders := orchestraRunBuildProviders
	origBackendFactory := orchestraRunBackendFactory
	origExecutePipeline := orchestraRunExecutePipeline
	t.Cleanup(func() {
		orchestraRunLoadConfig = origLoadConfig
		orchestraRunBuildProviders = origBuildProviders
		orchestraRunBackendFactory = origBackendFactory
		orchestraRunExecutePipeline = origExecutePipeline
	})

	orchestraRunLoadConfig = func(globalFlags) (*config.HarnessConfig, error) {
		return &config.HarnessConfig{
			Orchestra: config.OrchestraConf{
				Judge: "claude",
				Providers: map[string]config.ProviderEntry{
					"claude": {Binary: "claude"},
					"codex":  {Binary: "codex", Args: []string{"exec"}},
				},
			},
		}, nil
	}
	orchestraRunBuildProviders = buildProviderConfigsForRuntime
	orchestraRunBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return noopExecutionBackend{} }

	var captured orchestra.SubprocessPipelineConfig
	orchestraRunExecutePipeline = func(_ context.Context, cfg orchestra.SubprocessPipelineConfig) (*orchestra.OrchestraResult, error) {
		captured = cfg
		return successfulDebateRunResult(cfg.Providers[0].Name), nil
	}

	err := runSubprocessPipeline(context.Background(), "topic", "debate", []string{"codex"}, "fast", 120, false, "", false, false)
	require.NoError(t, err)
	assert.Equal(t, "codex", captured.Judge.Name)
	require.Len(t, captured.Providers, 1)
	assert.Equal(t, "codex", captured.Providers[0].Name)
}

func TestRunSubprocessPipeline_AppliesRuntimeCodexQualityAndEffort(t *testing.T) {
	installRuntimeCodexCatalogFixture(t)
	origLoadConfig := orchestraRunLoadConfig
	origBuildProviders := orchestraRunBuildProviders
	origBackendFactory := orchestraRunBackendFactory
	origExecutePipeline := orchestraRunExecutePipeline
	t.Cleanup(func() {
		orchestraRunLoadConfig = origLoadConfig
		orchestraRunBuildProviders = origBuildProviders
		orchestraRunBackendFactory = origBackendFactory
		orchestraRunExecutePipeline = origExecutePipeline
	})

	orchestraRunLoadConfig = func(flags globalFlags) (*config.HarnessConfig, error) {
		cfg := config.DefaultFullConfig("run-quality")
		cfg.Platforms = []string{"codex"}
		cfg.Quality.Default = "balanced"
		cfg.Orchestra.Providers["codex"] = managedCodexProviderForTest(cfg.Quality)
		effective := applyRuntimeHarnessOverrides(effectiveHarnessConfig{Config: cfg}, flags)
		return effective.Config, nil
	}
	orchestraRunBuildProviders = buildProviderConfigsForRuntime
	orchestraRunBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return noopExecutionBackend{} }

	var captured orchestra.SubprocessPipelineConfig
	orchestraRunExecutePipeline = func(_ context.Context, cfg orchestra.SubprocessPipelineConfig) (*orchestra.OrchestraResult, error) {
		captured = cfg
		return successfulDebateRunResult(cfg.Providers[0].Name), nil
	}

	ctx := withGlobalFlags(context.Background(), globalFlags{Quality: "ultra", Effort: config.CodexEffortMax})
	err := runSubprocessPipeline(ctx, "topic", "debate", []string{"codex"}, "fast", 120, false, "", false, false)
	require.NoError(t, err)
	require.Len(t, captured.Providers, 1)
	assertCodexProfileInArgs(t, captured.Providers[0].Args, config.CodexSolModel, config.CodexEffortMax)
	assertCodexProfileInArgs(t, captured.Providers[0].PaneArgs, config.CodexSolModel, config.CodexEffortMax)
}
