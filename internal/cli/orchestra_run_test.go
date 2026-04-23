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

	orchestraRunLoadConfig = func() (*config.OrchestraConf, error) {
		return &config.OrchestraConf{
			TimeoutSeconds: 240,
			Providers: map[string]config.ProviderEntry{
				"claude": {Binary: "claude"},
			},
		}, nil
	}
	orchestraRunBuildProviders = buildProviderConfigs
	orchestraRunBackendFactory = func() orchestra.ExecutionBackend { return noopExecutionBackend{} }

	var captured orchestra.SubprocessPipelineConfig
	orchestraRunExecutePipeline = func(_ context.Context, cfg orchestra.SubprocessPipelineConfig) (*orchestra.OrchestraResult, error) {
		captured = cfg
		return &orchestra.OrchestraResult{Merged: "ok", Summary: "done"}, nil
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

	orchestraRunLoadConfig = func() (*config.OrchestraConf, error) {
		return &config.OrchestraConf{
			TimeoutSeconds: 240,
			Providers: map[string]config.ProviderEntry{
				"claude": {Binary: "claude"},
			},
		}, nil
	}
	orchestraRunBuildProviders = buildProviderConfigs
	orchestraRunBackendFactory = func() orchestra.ExecutionBackend { return noopExecutionBackend{} }

	var captured orchestra.SubprocessPipelineConfig
	orchestraRunExecutePipeline = func(_ context.Context, cfg orchestra.SubprocessPipelineConfig) (*orchestra.OrchestraResult, error) {
		captured = cfg
		return &orchestra.OrchestraResult{Merged: "ok", Summary: "done"}, nil
	}

	err := runSubprocessPipeline(context.Background(), "topic", "debate", []string{"claude"}, "standard", 90, true, "", false, false)
	require.NoError(t, err)
	assert.Equal(t, 90, captured.TimeoutSeconds)
}
