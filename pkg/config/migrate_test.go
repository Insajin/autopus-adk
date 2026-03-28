package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrateOrchestraConfig_CodexPromptViaArgsFalse verifies R4:
// when auto update is executed and codex provider has PromptViaArgs=false,
// MigrateOrchestraConfig must set it to true.
func TestMigrateOrchestraConfig_CodexPromptViaArgsFalse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		initialCfg       *HarnessConfig
		expectPromptTrue bool
	}{
		{
			name: "codex PromptViaArgs=false is migrated to true",
			initialCfg: &HarnessConfig{
				Mode:        ModeFull,
				ProjectName: "test-project",
				Platforms:   []string{"claude-code", "codex"},
				Orchestra: OrchestraConf{
					Enabled: true,
					Providers: map[string]ProviderEntry{
						"claude": {Binary: "claude", Args: []string{"--print"}},
						"codex":  {Binary: "codex", Args: []string{"--quiet"}, PromptViaArgs: false},
					},
					Commands: map[string]CommandEntry{
						"review": {Strategy: "debate", Providers: []string{"claude", "codex", "gemini"}},
					},
				},
			},
			expectPromptTrue: true,
		},
		{
			name: "codex PromptViaArgs=true is preserved",
			initialCfg: &HarnessConfig{
				Mode:        ModeFull,
				ProjectName: "test-project",
				Platforms:   []string{"claude-code", "codex"},
				Orchestra: OrchestraConf{
					Enabled: true,
					Providers: map[string]ProviderEntry{
						"codex": {Binary: "codex", Args: []string{"--quiet"}, PromptViaArgs: true},
					},
					Commands: map[string]CommandEntry{},
				},
			},
			expectPromptTrue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			changed, err := MigrateOrchestraConfig(tt.initialCfg)
			require.NoError(t, err)

			codex, ok := tt.initialCfg.Orchestra.Providers["codex"]
			require.True(t, ok, "codex provider must exist after migration")
			assert.Equal(t, tt.expectPromptTrue, codex.PromptViaArgs,
				"codex PromptViaArgs must be %v after migration", tt.expectPromptTrue)

			if !tt.initialCfg.Orchestra.Providers["codex"].PromptViaArgs {
				assert.True(t, changed, "changed must be true when migration was applied (R4)")
			}
		})
	}
}

// TestMigrateOrchestraConfig_CodexMissingFromCommands verifies R5:
// when codex is in platforms but missing from orchestra command providers,
// MigrateOrchestraConfig must add codex to each command's providers.
func TestMigrateOrchestraConfig_CodexMissingFromCommands(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"claude-code", "codex"},
		Orchestra: OrchestraConf{
			Enabled: true,
			Providers: map[string]ProviderEntry{
				"claude": {Binary: "claude", Args: []string{"--print"}},
				"codex":  {Binary: "codex", Args: []string{"--quiet"}, PromptViaArgs: true},
				"gemini": {Binary: "gemini", Args: []string{}, PromptViaArgs: true},
			},
			Commands: map[string]CommandEntry{
				"review": {Strategy: "debate", Providers: []string{"claude", "codex", "gemini"}},
				"plan":   {Strategy: "consensus", Providers: []string{"claude", "gemini"}},
				"secure": {Strategy: "consensus", Providers: []string{"claude", "gemini"}},
			},
		},
	}

	changed, err := MigrateOrchestraConfig(cfg)
	require.NoError(t, err)
	assert.True(t, changed, "changed must be true when codex was added to commands (R5)")

	for _, cmdName := range []string{"review", "plan", "secure"} {
		entry, ok := cfg.Orchestra.Commands[cmdName]
		require.True(t, ok, "command %q must exist", cmdName)
		assert.Contains(t, entry.Providers, "codex",
			"command %q must include codex in providers after migration (R5)", cmdName)
	}
}

// TestMigrateOrchestraConfig_OrchestraDisabled verifies that MigrateOrchestraConfig
// returns (false, nil) immediately when Orchestra.Enabled is false.
func TestMigrateOrchestraConfig_OrchestraDisabled(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"claude-code"},
		Orchestra:   OrchestraConf{Enabled: false},
	}

	changed, err := MigrateOrchestraConfig(cfg)
	require.NoError(t, err)
	assert.False(t, changed, "changed must be false when orchestra is disabled")
}

// TestMigrateOrchestraConfig_NilProvidersWithKnownPlatform verifies that
// MigrateOrchestraConfig initializes nil Providers map and adds entries
// for known platforms.
func TestMigrateOrchestraConfig_NilProvidersWithKnownPlatform(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"claude-code"},
		Orchestra: OrchestraConf{
			Enabled:   true,
			Providers: nil,
		},
	}

	changed, err := MigrateOrchestraConfig(cfg)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Contains(t, cfg.Orchestra.Providers, "claude")
}

// TestMigrateOrchestraConfig_UnknownPlatformSkipped verifies that unknown
// platform names are simply skipped.
func TestMigrateOrchestraConfig_UnknownPlatformSkipped(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"unknown-platform"},
		Orchestra: OrchestraConf{
			Enabled:   true,
			Providers: map[string]ProviderEntry{},
		},
	}

	changed, err := MigrateOrchestraConfig(cfg)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Empty(t, cfg.Orchestra.Providers)
}

// TestMigrateOrchestraConfig_NilCommandsReturnsEarly verifies that when
// Orchestra.Commands is nil, MigrateOrchestraConfig returns without error.
func TestMigrateOrchestraConfig_NilCommandsReturnsEarly(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{},
		Orchestra: OrchestraConf{
			Enabled:   true,
			Providers: map[string]ProviderEntry{"claude": {Binary: "claude"}},
			Commands:  nil,
		},
	}

	changed, err := MigrateOrchestraConfig(cfg)
	require.NoError(t, err)
	assert.False(t, changed)
}

// TestMigrateOrchestraConfig_GeminiCliMapsToGeminiProvider verifies that the
// platform "gemini-cli" is correctly mapped to the "gemini" orchestra provider.
func TestMigrateOrchestraConfig_GeminiCliMapsToGeminiProvider(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"gemini-cli"},
		Orchestra: OrchestraConf{
			Enabled:   true,
			Providers: map[string]ProviderEntry{},
			Commands:  nil,
		},
	}

	changed, err := MigrateOrchestraConfig(cfg)
	require.NoError(t, err)
	assert.True(t, changed, "changed must be true when gemini provider is added for gemini-cli")

	gemini, ok := cfg.Orchestra.Providers["gemini"]
	require.True(t, ok, "gemini provider must exist after migrating gemini-cli platform")
	assert.Equal(t, "gemini", gemini.Binary, "gemini provider Binary must be 'gemini'")
	assert.True(t, gemini.PromptViaArgs, "gemini provider must have PromptViaArgs=true")
}

// TestMigrateOrchestraConfig_MixedKnownUnknownPlatforms verifies that known
// platforms produce provider entries while unknown platforms are silently skipped.
func TestMigrateOrchestraConfig_MixedKnownUnknownPlatforms(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"claude-code", "unknown-tool", "gemini-cli"},
		Orchestra: OrchestraConf{
			Enabled:   true,
			Providers: map[string]ProviderEntry{},
			Commands:  nil,
		},
	}

	changed, err := MigrateOrchestraConfig(cfg)
	require.NoError(t, err)
	assert.True(t, changed, "changed must be true when known providers are added")

	// Known platforms must produce providers.
	assert.Contains(t, cfg.Orchestra.Providers, "claude", "claude-code must map to claude provider")
	assert.Contains(t, cfg.Orchestra.Providers, "gemini", "gemini-cli must map to gemini provider")

	// Unknown platform must not produce a provider.
	assert.NotContains(t, cfg.Orchestra.Providers, "unknown-tool",
		"unknown platforms must not produce provider entries")
}

// TestMigrateOrchestraConfig_AlreadyCorrectConfigNoChange verifies that a config
// that already satisfies all migration conditions returns changed=false.
func TestMigrateOrchestraConfig_AlreadyCorrectConfigNoChange(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"claude-code", "opencode"},
		Orchestra: OrchestraConf{
			Enabled: true,
			Providers: map[string]ProviderEntry{
				// opencode already present with correct args (post-migration state).
				"claude":   {Binary: "claude", Args: []string{"--print"}},
				"opencode": {Binary: "opencode", Args: []string{"run", "-m", "openai/gpt-5.4"}, PromptViaArgs: false},
			},
			Commands: map[string]CommandEntry{
				// Both providers already listed in every command.
				"review": {Strategy: "debate", Providers: []string{"claude", "opencode"}},
				"plan":   {Strategy: "consensus", Providers: []string{"claude", "opencode"}},
			},
		},
	}

	changed, err := MigrateOrchestraConfig(cfg)
	require.NoError(t, err)
	assert.False(t, changed, "already-correct config must return changed=false")
}


