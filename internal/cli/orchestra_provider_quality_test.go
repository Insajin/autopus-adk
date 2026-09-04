package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestLoadHarnessConfigForDirUsesPersistedCodexProviderQuality(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := config.DefaultFullConfig("provider-quality")
	cfg.Platforms = []string{"claude-code", "codex"}
	cfg.Quality.Default = "balanced"
	cfg.Quality.Providers = map[string]string{
		config.QualityProviderClaude: "balanced",
		config.QualityProviderCodex:  "ultra",
	}
	cfg.Orchestra.Providers["codex"] = managedCodexProviderForTest(cfg.Quality)
	require.NoError(t, config.Save(dir, cfg))

	effective, err := loadHarnessConfigForDir(dir, globalFlags{})
	require.NoError(t, err)
	assert.Equal(t, "balanced", effective.Quality.Default)
	assert.Equal(t, "ultra", effective.Quality.EffectiveMode(config.QualityProviderCodex))
	assertCodexProfileInArgs(t, effective.Orchestra.Providers["codex"].Args, config.CodexAstraModel, config.CodexEffortMax)
}

func TestRuntimeGlobalQualityOverridesPersistedProviderModes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := config.DefaultFullConfig("runtime-provider-quality")
	cfg.Platforms = []string{"claude-code", "codex"}
	cfg.Quality.Default = "balanced"
	cfg.Quality.Providers = map[string]string{
		config.QualityProviderClaude: "ultra",
		config.QualityProviderCodex:  "ultra",
	}
	cfg.Orchestra.Providers["codex"] = managedCodexProviderForTest(cfg.Quality)
	require.NoError(t, config.Save(dir, cfg))
	before, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)

	effective, err := loadHarnessConfigForDir(dir, globalFlags{Quality: "balanced"})
	require.NoError(t, err)
	assert.Equal(t, "balanced", effective.Quality.Default)
	assert.Empty(t, effective.Quality.Providers)
	assertCodexProfileInArgs(t, effective.Orchestra.Providers["codex"].Args, config.CodexAstraModel, config.CodexEffortXHigh)

	disk, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)
	assert.Equal(t, before, disk)
}
