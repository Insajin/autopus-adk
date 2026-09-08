package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexAgentConcurrency_DefaultAndOverride(t *testing.T) {
	t.Parallel()

	assert.Equal(t, CodexAgentConcurrencyDefault,
		DefaultFullConfig("concurrency").CodexAgentConcurrency())

	// An autopus.yaml written before this setting existed decodes to zero and
	// must resolve to the default rather than to "no workers".
	legacy := DefaultFullConfig("concurrency")
	legacy.Codex.Agents.MaxConcurrentThreads = 0
	assert.Equal(t, CodexAgentConcurrencyDefault, legacy.CodexAgentConcurrency())

	explicit := DefaultFullConfig("concurrency")
	explicit.Codex.Agents.MaxConcurrentThreads = 8
	assert.Equal(t, 8, explicit.CodexAgentConcurrency())
}

func TestCodexAgentConcurrency_ValidationBounds(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		requested int
		wantErr   bool
	}{
		{name: "unset", requested: 0},
		{name: "minimum", requested: CodexAgentConcurrencyMin},
		{name: "maximum", requested: CodexAgentConcurrencyMax},
		{name: "above maximum", requested: CodexAgentConcurrencyMax + 1, wantErr: true},
		{name: "negative", requested: -4, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := DefaultFullConfig("concurrency")
			cfg.Codex.Agents.MaxConcurrentThreads = tc.requested
			err := cfg.Validate()
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "codex.agents.max_concurrent_threads")
				return
			}
			require.NoError(t, err)
		})
	}
}

// A user who raises the ceiling must keep it: setup and update both round the
// config through Load and Save, so a regenerated file that drops back to the
// default would silently undo the choice.
func TestCodexAgentConcurrency_ExplicitValueSurvivesRegeneration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	raw := "mode: full\nproject_name: concurrency\nplatforms:\n  - codex\n" +
		"codex:\n  agents:\n    max_concurrent_threads: 8\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(raw), 0o600))

	loaded, err := Load(dir)
	require.NoError(t, err)
	require.Equal(t, 8, loaded.CodexAgentConcurrency())

	require.NoError(t, Save(dir, loaded))

	reloaded, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 8, reloaded.CodexAgentConcurrency())
	assert.Equal(t, 8, reloaded.Codex.Agents.MaxConcurrentThreads)
}
