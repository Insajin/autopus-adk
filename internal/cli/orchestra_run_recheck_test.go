package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func recheckHarnessConfig(globalFlags) (*config.HarnessConfig, error) {
	return &config.HarnessConfig{
		Orchestra: config.OrchestraConf{
			TimeoutSeconds: 120,
			Providers: map[string]config.ProviderEntry{
				"claude": {Binary: "claude"},
				"codex":  {Binary: "codex"},
			},
		},
	}, nil
}

// `auto orchestra run --strategy recheck` must reach the legacy engine with the
// single requested provider and must not spawn the schema-guided debate
// pipeline — recheck has no peers to schema-fence.
func TestRunSubprocessPipeline_RecheckRoutesToEngineNotPipeline(t *testing.T) {
	origLoadConfig := orchestraRunLoadConfig
	origBuildProviders := orchestraRunBuildProviders
	origBackendFactory := orchestraRunBackendFactory
	origExecutePipeline := orchestraRunExecutePipeline
	origExecute := runOrchestraExecute
	t.Cleanup(func() {
		orchestraRunLoadConfig = origLoadConfig
		orchestraRunBuildProviders = origBuildProviders
		orchestraRunBackendFactory = origBackendFactory
		orchestraRunExecutePipeline = origExecutePipeline
		runOrchestraExecute = origExecute
	})

	orchestraRunLoadConfig = recheckHarnessConfig
	orchestraRunBuildProviders = buildProviderConfigsForRuntime
	orchestraRunBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend {
		return noopExecutionBackend{}
	}

	pipelineCalled := false
	orchestraRunExecutePipeline = func(context.Context, orchestra.SubprocessPipelineConfig) (*orchestra.OrchestraResult, error) {
		pipelineCalled = true
		return nil, nil
	}

	var captured orchestra.OrchestraConfig
	runOrchestraExecute = func(_ context.Context, cfg orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		captured = cfg
		return &orchestra.OrchestraResult{
			Strategy: orchestra.StrategyRecheck,
			Responses: []orchestra.ProviderResponse{
				{Provider: cfg.Providers[0].Name, Output: "first pass"},
				{Provider: cfg.Providers[0].Name, Output: "re-derived"},
			},
			Merged:  "re-derived",
			Summary: "재검토: claude 2라운드, 답변 수정",
		}, nil
	}

	err := runSubprocessPipeline(
		orchestraRunTestCmd(context.Background()),
		"topic", "recheck", []string{"claude"}, "standard", 120, false, "", false, false, false,
	)
	require.NoError(t, err)
	assert.False(t, pipelineCalled, "recheck must not invoke the debate subprocess pipeline")
	assert.Equal(t, orchestra.StrategyRecheck, captured.Strategy)
	require.Len(t, captured.Providers, 1, "recheck is a single-participant strategy")
	assert.Equal(t, "claude", captured.Providers[0].Name)
}

// The strategy guard is the only thing standing between a typo and a silent
// fallback to some other strategy, so it must still reject unknown values.
func TestRunSubprocessPipeline_RejectsUnknownStrategy(t *testing.T) {
	origLoadConfig := orchestraRunLoadConfig
	t.Cleanup(func() { orchestraRunLoadConfig = origLoadConfig })
	orchestraRunLoadConfig = recheckHarnessConfig

	err := runSubprocessPipeline(
		orchestraRunTestCmd(context.Background()),
		"topic", "recheckk", []string{"claude"}, "standard", 120, false, "", false, false, false,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recheck")
}
