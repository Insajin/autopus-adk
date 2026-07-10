package codex

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateConfig_CatalogDowngradesEffortOnSameModel(t *testing.T) {
	t.Parallel()

	a := NewWithRoot(t.TempDir())
	a.codexCatalogProbed = true
	a.codexCatalogJSON = []byte(`{"models":[{"slug":"gpt-5.6-sol","supported_reasoning_levels":[{"effort":"xhigh"},{"effort":"max"}]}]}`)
	var warnings bytes.Buffer
	a.codexFallbackWriter = &warnings
	cfg := config.DefaultFullConfig("catalog-project")
	cfg.Quality.Default = "ultra"

	files, err := a.generateConfig(cfg)
	require.NoError(t, err)
	root := strings.SplitN(string(files[0].Content), "[agents]", 2)[0]
	assert.Contains(t, root, `model = "gpt-5.6-sol"`)
	assert.Contains(t, root, `model_reasoning_effort = "max"`)
	assert.Contains(t, warnings.String(), "requested=gpt-5.6-sol/ultra")
	assert.Contains(t, warnings.String(), "selected=gpt-5.6-sol/max")
	assert.Contains(t, warnings.String(), "reason=effort_unavailable")
	assert.Equal(t, 1, strings.Count(warnings.String(), "reason=effort_unavailable"))
}

func TestGenerateConfig_CatalogFallsBackToLegacyModel(t *testing.T) {
	t.Parallel()

	a := NewWithRoot(t.TempDir())
	a.codexCatalogProbed = true
	a.codexCatalogJSON = []byte(`{"models":[{"slug":"gpt-5.5","supported_reasoning_levels":[{"effort":"xhigh"}]}]}`)
	cfg := config.DefaultFullConfig("legacy-project")
	cfg.Quality.Default = "ultra"

	files, err := a.generateConfig(cfg)
	require.NoError(t, err)
	root := strings.SplitN(string(files[0].Content), "[agents]", 2)[0]
	assert.Contains(t, root, `model = "gpt-5.5"`)
	assert.Contains(t, root, `model_reasoning_effort = "xhigh"`)
}

func TestGenerateConfig_CatalogUnknownUsesLegacyModel(t *testing.T) {
	t.Parallel()

	a := NewWithRoot(t.TempDir())
	a.codexCatalogProbed = true
	var warnings bytes.Buffer
	a.codexFallbackWriter = &warnings
	cfg := config.DefaultFullConfig("unknown-catalog-project")
	cfg.Quality.Default = "ultra"

	files, err := a.generateConfig(cfg)
	require.NoError(t, err)
	root := strings.SplitN(string(files[0].Content), "[agents]", 2)[0]
	assert.Contains(t, root, `model = "gpt-5.5"`)
	assert.Contains(t, root, `model_reasoning_effort = "xhigh"`)
	assert.Contains(t, warnings.String(), "reason=catalog_unknown")
}

func TestGenerateConfig_CatalogUsesRuntimeDefaultWhenNoCompatibleModel(t *testing.T) {
	t.Parallel()

	a := NewWithRoot(t.TempDir())
	a.codexCatalogProbed = true
	a.codexCatalogJSON = []byte(`{"models":[{"slug":"other-model","supported_reasoning_levels":[{"effort":"medium"}]}]}`)
	cfg := config.DefaultFullConfig("runtime-default-project")

	files, err := a.generateConfig(cfg)
	require.NoError(t, err)
	root := strings.SplitN(string(files[0].Content), "[agents]", 2)[0]
	assert.NotContains(t, root, "\nmodel =")
	assert.NotContains(t, root, "model_reasoning_effort")
}

func TestGenerateConfig_RuntimeDefaultStillPreservesUserModel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := NewWithRoot(dir)
	a.codexCatalogProbed = true
	a.codexCatalogJSON = []byte(`{"models":[{"slug":"other-model","supported_reasoning_levels":[{"effort":"medium"}]}]}`)
	configPath := filepath.Join(dir, ".codex", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0755))
	require.NoError(t, os.WriteFile(configPath, []byte("model = \"custom-model\"\nmodel_reasoning_effort = \"ultra\"\n"), 0644))

	files, err := a.generateConfig(config.DefaultFullConfig("preserve-project"))
	require.NoError(t, err)
	root := strings.SplitN(string(files[0].Content), "[agents]", 2)[0]
	assert.Contains(t, root, `model = "custom-model"`)
	assert.Contains(t, root, `model_reasoning_effort = "ultra"`)
}

func TestCodexRenderContext_ResolvesAgentModelWithDeclaredEffort(t *testing.T) {
	t.Parallel()

	a := NewWithRoot(t.TempDir())
	a.codexCatalogProbed = true
	a.codexCatalogJSON = []byte(`{"models":[{"slug":"gpt-5.6-terra","supported_reasoning_levels":[{"effort":"high"}]}]}`)
	var warnings bytes.Buffer
	a.codexFallbackWriter = &warnings
	cfg := config.DefaultFullConfig("tuple-project")
	data := codexRenderContext{HarnessConfig: cfg, adapter: a}

	assert.Equal(t, config.CodexTerraModel, data.CodexAgentModel("reviewer", "sonnet", "high"))
	assert.Equal(t, config.CodexEffortHigh, data.CodexAgentEffort("reviewer", "sonnet", "high"))
	assert.Empty(t, warnings.String())
}

func TestGenerateAgents_AppliesCatalogFallbackProfiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		catalog       string
		wantModel     string
		wantEffort    string
		wantReason    string
		wantOmissions bool
	}{
		{
			name:       "legacy fallback",
			catalog:    `{"models":[{"slug":"gpt-5.5","supported_reasoning_levels":[{"effort":"xhigh"}]}]}`,
			wantModel:  config.CodexLegacyModel,
			wantEffort: config.CodexEffortXHigh,
			wantReason: "model_unavailable",
		},
		{
			name:       "effort downgrade",
			catalog:    `{"models":[{"slug":"gpt-5.6-sol","supported_reasoning_levels":[{"effort":"xhigh"}]}]}`,
			wantModel:  config.CodexSolModel,
			wantEffort: config.CodexEffortXHigh,
			wantReason: "effort_unavailable",
		},
		{
			name:          "runtime default",
			catalog:       `{"models":[{"slug":"other-model","supported_reasoning_levels":[{"effort":"medium"}]}]}`,
			wantReason:    "runtime_default",
			wantOmissions: true,
		},
		{
			name:       "catalog unknown",
			wantModel:  config.CodexLegacyModel,
			wantEffort: config.CodexEffortXHigh,
			wantReason: "catalog_unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a := NewWithRoot(t.TempDir())
			a.codexCatalogProbed = true
			a.codexCatalogJSON = []byte(tt.catalog)
			var warnings bytes.Buffer
			a.codexFallbackWriter = &warnings
			cfg := config.DefaultFullConfig("agent-fallback-project")
			cfg.Quality.Default = "ultra"

			files, err := a.generateAgents(cfg)
			require.NoError(t, err)
			var executor string
			for _, file := range files {
				if file.TargetPath == filepath.Join(".codex", "agents", "executor.toml") {
					executor = string(file.Content)
					break
				}
			}
			require.NotEmpty(t, executor)
			if tt.wantOmissions {
				assert.NotContains(t, executor, "\nmodel =")
				assert.NotContains(t, executor, "model_reasoning_effort")
				assert.Contains(t, warnings.String(), "reason="+tt.wantReason)
				return
			}
			assert.Contains(t, executor, `model = "`+tt.wantModel+`"`)
			assert.Contains(t, executor, `model_reasoning_effort = "`+tt.wantEffort+`"`)
			assert.Contains(t, warnings.String(), "reason="+tt.wantReason)
		})
	}
}
