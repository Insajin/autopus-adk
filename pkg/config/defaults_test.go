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
	assert.False(t, gemini.PromptViaArgs, "gemini provider must have PromptViaArgs=false")
}

func TestDefaultFullConfig_OtherProvidersPromptViaArgsFalse(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("test-project")
	require.NotNil(t, cfg)

	claude, ok := cfg.Orchestra.Providers["claude"]
	require.True(t, ok, "claude provider must exist")
	assert.False(t, claude.PromptViaArgs, "claude provider must have PromptViaArgs=false")
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

	// Spot-check balanced preset: planner=opus, executor=sonnet, validator=sonnet.
	assert.Equal(t, "opus", balanced.Agents["planner"])
	assert.Equal(t, "sonnet", balanced.Agents["executor"])
	assert.Equal(t, "sonnet", balanced.Agents["validator"])
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

// TestDefaultFullConfig_CodexPromptViaArgs verifies codex uses stdin pipe
// instead of CLI args (PromptViaArgs=false) with exec-mode args.
func TestDefaultFullConfig_CodexPromptViaArgs(t *testing.T) {
	t.Parallel()
	cfg := DefaultFullConfig("test-project")
	require.NotNil(t, cfg)

	codex, ok := cfg.Orchestra.Providers["codex"]
	require.True(t, ok, "codex provider must exist in default full config")
	assert.False(t, codex.PromptViaArgs, "codex provider must have PromptViaArgs=false")
	assert.Equal(t, []string{"exec", "--full-auto", "-m", CodexFrontierModel}, codex.Args,
		"codex provider must have correct exec-mode args")
	assert.Equal(t, CodexOrchestraTimeoutSeconds, codex.Subprocess.Timeout,
		"codex provider must have a longer default orchestra timeout")
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

// TestDefaultFullConfig_ClaudeProviderTimeout verifies claude has a per-provider
// subprocess timeout that exceeds the global orchestra timeout, preventing the
// 4-minute cutoff observed in issue #55 when running `--model opus --effort high`.
func TestDefaultFullConfig_ClaudeProviderTimeout(t *testing.T) {
	t.Parallel()
	cfg := DefaultFullConfig("test-project")
	require.NotNil(t, cfg)

	claude, ok := cfg.Orchestra.Providers["claude"]
	require.True(t, ok, "claude provider must exist")

	assert.Equal(t, ClaudeOrchestraTimeoutSeconds, claude.Subprocess.Timeout,
		"claude provider must declare a per-provider subprocess timeout")
	assert.Greater(t, claude.Subprocess.Timeout, cfg.Orchestra.TimeoutSeconds,
		"claude per-provider timeout must exceed the global orchestra timeout")
}

// TestDefaultFullConfig_ClaudeEffortHigh verifies claude defaults to --effort high
// (not max) for spec review's structured-output workload. See issue #55 for the
// rationale: max-effort reasoning routinely exceeded 4 minutes on opus.
func TestDefaultFullConfig_ClaudeEffortHigh(t *testing.T) {
	t.Parallel()
	cfg := DefaultFullConfig("test-project")
	require.NotNil(t, cfg)

	claude, ok := cfg.Orchestra.Providers["claude"]
	require.True(t, ok, "claude provider must exist")

	assert.Contains(t, claude.Args, "high",
		"claude default args must use --effort high (issue #55)")
	assert.NotContains(t, claude.Args, "max",
		"claude default args must not use --effort max (issue #55)")
	assert.Contains(t, claude.PaneArgs, "high",
		"claude default pane args must use --effort high (issue #55)")
	assert.NotContains(t, claude.PaneArgs, "max",
		"claude default pane args must not use --effort max (issue #55)")
}

// TestDefaultFullConfig_CodexExistsNoOpencode verifies codex exists and opencode
// is absent from the default config.
func TestDefaultFullConfig_CodexExistsNoOpencode(t *testing.T) {
	t.Parallel()
	cfg := DefaultFullConfig("test-project")
	require.NotNil(t, cfg)

	_, hasCodex := cfg.Orchestra.Providers["codex"]
	assert.True(t, hasCodex, "codex provider must exist in default config")

	_, hasOpencode := cfg.Orchestra.Providers["opencode"]
	assert.False(t, hasOpencode, "opencode provider must not exist in default config")
}
