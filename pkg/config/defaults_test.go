package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultFullConfig_GeminiPromptViaArgs(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("test-project")
	require.NotNil(t, cfg)

	gemini, ok := cfg.Orchestra.Providers["gemini"]
	require.True(t, ok, "gemini provider must exist in default full config")
	assert.True(t, gemini.PromptViaArgs, "gemini provider must have PromptViaArgs=true")
}

func TestDefaultFullConfig_OtherProvidersPromptViaArgsFalse(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("test-project")
	require.NotNil(t, cfg)

	claude, ok := cfg.Orchestra.Providers["claude"]
	require.True(t, ok, "claude provider must exist")
	assert.False(t, claude.PromptViaArgs, "claude provider must have PromptViaArgs=false")

	codex, ok := cfg.Orchestra.Providers["codex"]
	require.True(t, ok, "codex provider must exist")
	assert.False(t, codex.PromptViaArgs, "codex provider must have PromptViaArgs=false")
}

func TestDefaultFullConfig_QualityPresets(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("test-project")
	require.NotNil(t, cfg)

	// Default preset name must be "balanced".
	assert.Equal(t, "balanced", cfg.Quality.Default)

	// Both "ultra" and "balanced" presets must exist.
	_, hasUltra := cfg.Quality.Presets["ultra"]
	require.True(t, hasUltra, "ultra preset must exist")

	_, hasBalanced := cfg.Quality.Presets["balanced"]
	require.True(t, hasBalanced, "balanced preset must exist")

	ultra := cfg.Quality.Presets["ultra"]
	balanced := cfg.Quality.Presets["balanced"]

	// Both presets must define the same number of agent mappings.
	assert.Len(t, balanced.Agents, len(ultra.Agents), "ultra and balanced must have the same number of agents")

	// Both presets must define the same set of agent keys.
	for agent := range ultra.Agents {
		_, exists := balanced.Agents[agent]
		assert.True(t, exists, "balanced preset must contain agent %q defined in ultra preset", agent)
	}

	// Spot-check balanced preset: planner=opus, executor=sonnet, validator=haiku.
	assert.Equal(t, "opus", balanced.Agents["planner"])
	assert.Equal(t, "sonnet", balanced.Agents["executor"])
	assert.Equal(t, "haiku", balanced.Agents["validator"])
}

func TestDefaultFullConfig_QualityUltraAllOpus(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("test-project")
	require.NotNil(t, cfg)

	ultra, ok := cfg.Quality.Presets["ultra"]
	require.True(t, ok, "ultra preset must exist")

	// Every agent in the ultra preset must map to "opus".
	for agent, model := range ultra.Agents {
		assert.Equal(t, "opus", model, "ultra preset agent %q must be opus", agent)
	}
}

func TestDefaultLiteConfig_NoQuality(t *testing.T) {
	t.Parallel()

	cfg := DefaultLiteConfig("test-project")
	require.NotNil(t, cfg)

	// Lite config must have zero-value Quality: empty Default and nil Presets.
	assert.Empty(t, cfg.Quality.Default, "lite config must have empty Quality.Default")
	assert.Nil(t, cfg.Quality.Presets, "lite config must have nil Quality.Presets")
}

// TestDefaultFullConfig_BrainstormCommand verifies that DefaultFullConfig includes
// a brainstorm command entry with debate strategy and all three providers.
func TestDefaultFullConfig_BrainstormCommand(t *testing.T) {
	t.Parallel()
	cfg := DefaultFullConfig("test-project")
	require.NotNil(t, cfg)

	brainstorm, ok := cfg.Orchestra.Commands["brainstorm"]
	require.True(t, ok, "brainstorm command must exist in orchestra commands")
	assert.Equal(t, "debate", brainstorm.Strategy)
	assert.Contains(t, brainstorm.Providers, "claude")
	assert.Contains(t, brainstorm.Providers, "codex")
	assert.Contains(t, brainstorm.Providers, "gemini")
}
