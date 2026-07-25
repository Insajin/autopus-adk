package codex

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestGenerateCodexSurfacesUseCodexProviderOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		global             string
		codex              string
		wantRootEffort     string
		wantPlannerEffort  string
		wantExecutorModel  string
		wantExecutorEffort string
	}{
		{
			name:               "global ultra codex balanced",
			global:             "ultra",
			codex:              "balanced",
			wantRootEffort:     config.CodexEffortXHigh,
			wantPlannerEffort:  config.CodexEffortXHigh,
			wantExecutorModel:  config.CodexTerraModel,
			wantExecutorEffort: config.CodexEffortMedium,
		},
		{
			name:               "global balanced codex ultra",
			global:             "balanced",
			codex:              "ultra",
			wantRootEffort:     config.CodexEffortUltra,
			wantPlannerEffort:  config.CodexEffortMax,
			wantExecutorModel:  config.CodexSolModel,
			wantExecutorEffort: config.CodexEffortXHigh,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			adapter := NewWithRoot(dir)
			cfg := config.DefaultFullConfig("provider-quality")
			cfg.Quality.Default = tt.global
			cfg.Quality.Providers = map[string]string{
				config.QualityProviderClaude: tt.global,
				config.QualityProviderCodex:  tt.codex,
			}
			cfg.Quality.SupervisorModelPolicy = config.SupervisorModelPolicyQuality

			configFiles, err := adapter.generateConfig(cfg)
			require.NoError(t, err)
			root := strings.SplitN(string(configFiles[0].Content), "[agents]", 2)[0]
			assert.Contains(t, root, `model = "`+config.CodexSolModel+`"`)
			assert.Contains(t, root, `model_reasoning_effort = "`+tt.wantRootEffort+`"`)

			agentFiles, err := adapter.generateAgents(cfg)
			require.NoError(t, err)
			byPath := make(map[string]string, len(agentFiles))
			for _, file := range agentFiles {
				byPath[file.TargetPath] = string(file.Content)
			}
			planner := byPath[filepath.Join(".codex", "agents", "planner.toml")]
			executor := byPath[filepath.Join(".codex", "agents", "executor.toml")]
			assert.Contains(t, planner, `model = "`+config.CodexSolModel+`"`)
			assert.Contains(t, planner, `model_reasoning_effort = "`+tt.wantPlannerEffort+`"`)
			assert.Contains(t, executor, `model = "`+tt.wantExecutorModel+`"`)
			assert.Contains(t, executor, `model_reasoning_effort = "`+tt.wantExecutorEffort+`"`)
		})
	}
}
