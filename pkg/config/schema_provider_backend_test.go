package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHarnessConfigValidateProviderBackend(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entry   ProviderEntry
		wantErr string
	}{
		{
			name:    "omp model required",
			entry:   ProviderEntry{Backend: ProviderBackendOMP},
			wantErr: "provider_model_required: claude",
		},
		{
			name: "omp tools fail closed",
			entry: ProviderEntry{
				Backend: ProviderBackendOMP,
				Model:   "anthropic/claude-fable-5-1:max",
				Tools:   []string{"read", "bash"},
			},
			wantErr: "provider_tools_invalid: claude",
		},
		{
			name:    "unknown backend rejected",
			entry:   ProviderEntry{Backend: "pane"},
			wantErr: "provider_backend_invalid: claude",
		},
		{
			name: "malformed omp model rejected",
			entry: ProviderEntry{
				Backend: ProviderBackendOMP,
				Model:   "anthropic/",
			},
			wantErr: "provider_model_required: claude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := validProviderBackendHarness(tt.entry)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}
}

func TestHarnessConfigValidateProviderBackendPreservesLegacyDefault(t *testing.T) {
	t.Parallel()

	cfg := validProviderBackendHarness(ProviderEntry{
		Binary: "claude",
		Args:   []string{"--print"},
	})

	require.NoError(t, cfg.Validate())
}

func TestProviderEntryEffectiveTools(t *testing.T) {
	t.Parallel()

	entry := ProviderEntry{}
	got := entry.EffectiveTools()
	assert.Equal(t, []string{"glob", "grep", "read"}, got, "default set is sorted like an explicit list")

	got[0] = "changed"
	assert.Equal(t, []string{"read", "grep", "glob"}, OMPReviewToolAllowlist)
	assert.Equal(t, []string{"glob", "grep", "read"}, (ProviderEntry{Tools: []string{}}).EffectiveTools())

	entry.Tools = []string{"read", "glob", "read", "grep", "glob"}
	assert.Equal(t, []string{"glob", "grep", "read"}, entry.EffectiveTools())
}

func TestHarnessConfigValidateProviderBackendAcceptsOMP(t *testing.T) {
	t.Parallel()

	cfg := validProviderBackendHarness(ProviderEntry{
		Backend: ProviderBackendOMP,
		Model:   "anthropic/claude-fable-5-1:max",
	})

	require.NoError(t, cfg.Validate())
	assert.Equal(t, []string{"glob", "grep", "read"}, cfg.Orchestra.Providers["claude"].EffectiveTools())
}

func validProviderBackendHarness(entry ProviderEntry) *HarnessConfig {
	return &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"claude-code"},
		Orchestra: OrchestraConf{
			Providers: map[string]ProviderEntry{"claude": entry},
		},
	}
}

// A provider that names an execution backend carries no CLI args on purpose;
// config migration must not mistake it for a blank entry and restore the
// shipped CLI defaults (which would silently spawn the external binary).
func TestMigrateOrchestraConfig_KeepsBackendProvidersUntouched(t *testing.T) {
	t.Parallel()

	claude := ProviderEntry{Backend: ProviderBackendOMP, Model: "anthropic/claude-fable-5-1:max", Subprocess: SubprocessProvConf{Timeout: 900}}
	codex := ProviderEntry{Backend: ProviderBackendOMP, Model: "openai-codex/gpt-6-astra:max", Subprocess: SubprocessProvConf{Timeout: 900}}
	gemini := ProviderEntry{Backend: ProviderBackendOMP, Model: "google-antigravity/gemini-3.1-pro:high", Subprocess: SubprocessProvConf{Timeout: 900}}
	cfg := &HarnessConfig{
		Platforms: []string{"claude-code", "codex", "antigravity-cli"},
		Quality:   QualityConf{Default: "ultra"},
		Orchestra: OrchestraConf{
			Enabled:   true,
			Providers: map[string]ProviderEntry{"claude": claude, "codex": codex, "gemini": gemini},
			Commands:  map[string]CommandEntry{"review": {Providers: []string{"claude", "codex", "gemini"}}},
		},
	}

	changed, err := MigrateOrchestraConfig(cfg)

	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, claude, cfg.Orchestra.Providers["claude"])
	assert.Equal(t, codex, cfg.Orchestra.Providers["codex"])
	assert.Equal(t, gemini, cfg.Orchestra.Providers["gemini"])
}
