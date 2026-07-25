package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestLoadHarnessConfig_InheritsParentOrchestraWhenChildOnlyNamesReviewProviders(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "autopus-desktop")
	require.NoError(t, os.MkdirAll(child, 0o755))
	writeSparseModuleConfigWithOnlyReviewProvider(t, child, "codex")

	parentCfg := config.DefaultFullConfig("root-workspace")
	parentCfg.Quality.Default = "ultra"
	require.NoError(t, config.Save(root, parentCfg))

	cfg, err := loadHarnessConfigForDir(child, globalFlags{})
	require.NoError(t, err)

	assert.Equal(t, []string{"codex"}, cfg.Spec.ReviewGate.Providers)
	assert.Equal(t, "--output-schema", cfg.Orchestra.Providers["codex"].Subprocess.SchemaFlag)
	assert.NotEmpty(t, cfg.Orchestra.Providers["codex"].Args)
	assert.Equal(t, "ultra", cfg.Quality.Default)
}

func TestLoadHarnessConfigOverlaysSparseChildProviderQualityOnParent(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "autopus-desktop")
	require.NoError(t, os.MkdirAll(child, 0o755))
	childPath := writeSparseModuleConfig(t, child)
	childData, err := os.ReadFile(childPath)
	require.NoError(t, err)
	childData = append(childData, []byte("quality:\n  providers:\n    codex: balanced\n")...)
	require.NoError(t, os.WriteFile(childPath, childData, 0o644))

	parentCfg := config.DefaultFullConfig("root-workspace")
	parentCfg.Quality.Default = "ultra"
	parentCfg.Quality.Providers = map[string]string{
		config.QualityProviderClaude: "balanced",
		config.QualityProviderCodex:  "ultra",
	}
	require.NoError(t, config.Save(root, parentCfg))

	cfg, err := loadHarnessConfigForDir(child, globalFlags{})
	require.NoError(t, err)

	assert.Equal(t, "ultra", cfg.Quality.Default)
	assert.Equal(t, "balanced", cfg.Quality.EffectiveMode(config.QualityProviderClaude))
	assert.Equal(t, "balanced", cfg.Quality.EffectiveMode(config.QualityProviderCodex))
	assert.Equal(t, "balanced", cfg.Quality.Providers[config.QualityProviderCodex])
	assert.Contains(t, cfg.Quality.Presets, "ultra")
	assert.Contains(t, cfg.Quality.Presets, "balanced")
}

func writeSparseModuleConfigWithOnlyReviewProvider(t *testing.T, dir string, provider string) {
	t.Helper()
	path := writeSparseModuleConfig(t, dir)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	updated := strings.Replace(string(data), "spec:\n  id_format: \"\"\n  ears_types: []\n", "spec:\n  id_format: \"\"\n  ears_types: []\n  review_gate:\n    providers:\n      - "+provider+"\n", 1)
	require.NoError(t, os.WriteFile(path, []byte(updated), 0o644))
}
