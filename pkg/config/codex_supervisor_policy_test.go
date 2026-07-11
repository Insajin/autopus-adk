package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultFullConfigInheritsCodexSupervisorModel(t *testing.T) {
	t.Parallel()
	cfg := DefaultFullConfig("test-project")

	assert.Equal(t, "inherit", cfg.Quality.SupervisorModelPolicy)
	assert.Empty(t, cfg.CodexSupervisorModel())
	assert.Empty(t, cfg.CodexSupervisorEffort())
}

func TestLegacyMissingSupervisorPolicyKeepsQualityManagedProfile(t *testing.T) {
	t.Parallel()
	cfg := DefaultFullConfig("test-project")
	cfg.Quality.SupervisorModelPolicy = ""
	cfg.Quality.Default = "ultra"

	assert.Equal(t, CodexSolModel, cfg.CodexSupervisorModel())
	assert.Equal(t, CodexEffortUltra, cfg.CodexSupervisorEffort())
}

func TestLoadLegacyYAMLMissingSupervisorPolicyKeepsQualityManagedProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := DefaultFullConfig("legacy-project")
	cfg.Quality.Default = "ultra"
	require.NoError(t, Save(dir, cfg))

	path := filepath.Join(dir, "autopus.yaml")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	raw = []byte(strings.Replace(string(raw), "    supervisor_model_policy: inherit\n", "", 1))
	require.NoError(t, os.WriteFile(path, raw, 0o644))

	loaded, err := LoadPreview(dir)
	require.NoError(t, err)
	assert.Empty(t, loaded.Quality.SupervisorModelPolicy)
	assert.Equal(t, SupervisorModelPolicyQuality, loaded.Quality.EffectiveSupervisorModelPolicy())
	assert.Equal(t, CodexSolModel, loaded.CodexSupervisorModel())
	assert.Equal(t, CodexEffortUltra, loaded.CodexSupervisorEffort())
}

func TestHarnessConfigRejectsUnknownSupervisorModelPolicy(t *testing.T) {
	t.Parallel()
	cfg := DefaultFullConfig("test-project")
	cfg.Quality.SupervisorModelPolicy = "forced"

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality.supervisor_model_policy")
}
