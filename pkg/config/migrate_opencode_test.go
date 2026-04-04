package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrateOrchestraConfig_OpencodeToCodex verifies full migration flow:
// opencode present in providers -> becomes codex with correct defaults.
func TestMigrateOrchestraConfig_OpencodeToCodex(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"claude-code", "opencode"},
		Orchestra: OrchestraConf{
			Enabled: true,
			Providers: map[string]ProviderEntry{
				"claude":   {Binary: "claude", Args: []string{"--print"}},
				"opencode": {Binary: "opencode", Args: []string{"run", "-m", "openai/gpt-5.4"}, PromptViaArgs: false},
			},
			Commands: map[string]CommandEntry{
				"review": {Strategy: "debate", Providers: []string{"claude", "opencode"}},
			},
		},
	}

	changed, err := MigrateOpencodeToCodex(cfg)
	require.NoError(t, err)
	assert.True(t, changed, "changed must be true when opencode is migrated to codex")

	// opencode entry must be removed.
	_, hasOpencode := cfg.Orchestra.Providers["opencode"]
	assert.False(t, hasOpencode, "opencode provider must be removed after migration")

	// codex entry must exist with correct settings.
	codex, hasCodex := cfg.Orchestra.Providers["codex"]
	require.True(t, hasCodex, "codex provider must exist after migration")
	assert.Equal(t, "codex", codex.Binary, "codex binary must be 'codex'")
	assert.Equal(t, []string{"exec", "--full-auto", "-m", "gpt-5.4"}, codex.Args)
	assert.False(t, codex.PromptViaArgs, "codex PromptViaArgs must be false")

	// Commands must reference codex instead of opencode.
	review := cfg.Orchestra.Commands["review"]
	assert.Contains(t, review.Providers, "codex",
		"command providers must include codex after migration")
	assert.NotContains(t, review.Providers, "opencode",
		"command providers must not include opencode after migration")
}

// TestMigrateOrchestraConfig_CodexAlreadyExists verifies that when
// codex provider already exists, opencode is removed and existing codex
// is preserved without duplication.
func TestMigrateOrchestraConfig_CodexAlreadyExists(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"claude-code", "opencode"},
		Orchestra: OrchestraConf{
			Enabled: true,
			Providers: map[string]ProviderEntry{
				"claude":   {Binary: "claude", Args: []string{"--print"}},
				"codex":    {Binary: "codex", Args: []string{"exec", "--quiet"}, PromptViaArgs: false},
				"opencode": {Binary: "opencode", Args: []string{"run", "-m", "openai/gpt-5.4"}},
			},
			Commands: map[string]CommandEntry{
				"plan": {Strategy: "consensus", Providers: []string{"claude", "codex", "opencode"}},
			},
		},
	}

	changed, err := MigrateOpencodeToCodex(cfg)
	require.NoError(t, err)

	// opencode must be removed.
	_, hasOpencode := cfg.Orchestra.Providers["opencode"]
	assert.False(t, hasOpencode, "opencode must be removed even when codex already exists")

	// codex must still exist (not duplicated).
	_, hasCodex := cfg.Orchestra.Providers["codex"]
	assert.True(t, hasCodex, "codex must still exist")

	// Commands must not list opencode.
	plan := cfg.Orchestra.Commands["plan"]
	assert.NotContains(t, plan.Providers, "opencode",
		"command providers must not include opencode after migration")
	assert.Contains(t, plan.Providers, "codex",
		"command providers must still include codex")

	_ = changed
}

// TestPlatformToProvider_Opencode verifies that the "opencode" platform
// correctly maps to the "codex" orchestra provider.
func TestPlatformToProvider_Opencode(t *testing.T) {
	t.Parallel()

	got := PlatformToProvider("opencode")
	assert.Equal(t, "codex", got,
		"platform 'opencode' must map to provider 'codex'")
}
