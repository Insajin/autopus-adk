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
