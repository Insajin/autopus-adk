package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestBuildReviewProvidersWithConfig_UsesRuntimeCodexQualityProfile(t *testing.T) {
	dir := t.TempDir()
	setFakeProviderOnPath(t, dir, "codex")

	cfg := config.DefaultFullConfig("spec-review-quality")
	cfg.Platforms = []string{"codex"}
	cfg.Quality.Default = "balanced"
	cfg.Orchestra.Providers = map[string]config.ProviderEntry{"codex": managedCodexProviderForTest(cfg.Quality)}
	effective := applyRuntimeHarnessOverrides(effectiveHarnessConfig{Config: cfg}, globalFlags{Quality: "ultra", Effort: config.CodexEffortMax})

	result := buildReviewProvidersWithConfig(effective.Config, []string{"codex"})
	require.Len(t, result, 1)
	assertCodexProfileInArgs(t, result[0].Args, config.CodexSolModel, config.CodexEffortMax)
	assertCodexProfileInArgs(t, result[0].PaneArgs, config.CodexSolModel, config.CodexEffortMax)
}

func TestBuildReviewProvidersWithConfig_UsesResolvedProviderSettings(t *testing.T) {
	dir := t.TempDir()
	setFakeProviderOnPath(t, dir, "gemini")

	cfg := config.DefaultFullConfig("test-project")
	cfg.Orchestra.Providers = map[string]config.ProviderEntry{
		"gemini": {
			Binary: "gemini",
			Args:   []string{"-m", "gemini-3.1-pro-preview", "-p", ""},
			Subprocess: config.SubprocessProvConf{
				Timeout: 300,
			},
		},
	}

	result := buildReviewProvidersWithConfig(cfg, []string{"gemini"})
	require.Len(t, result, 1)
	assert.Equal(t, "gemini", result[0].Name)
	assert.Equal(t, 300*time.Second, result[0].ExecutionTimeout)
	assert.Equal(t, defaultProviderStartupTimeout("gemini"), result[0].StartupTimeout)
}
