package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityConfEffectiveModeUsesProviderOverrideThenDefault(t *testing.T) {
	t.Parallel()

	quality := QualityConf{
		Default: "balanced",
		Providers: map[string]string{
			QualityProviderClaude: "ultra",
			QualityProviderCodex:  "balanced",
		},
	}

	assert.Equal(t, "ultra", quality.EffectiveMode(QualityProviderClaude))
	assert.Equal(t, "ultra", quality.EffectiveMode("claude-code"))
	assert.Equal(t, "balanced", quality.EffectiveMode(QualityProviderCodex))
	assert.Equal(t, "balanced", quality.EffectiveMode("unknown"))
}

func TestQualityConfEffectiveModeFallsBackToBalanced(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "balanced", (QualityConf{}).EffectiveMode(QualityProviderClaude))
	assert.Equal(t, "balanced", (QualityConf{}).EffectiveMode(QualityProviderCodex))
	assert.Equal(t, "balanced", (QualityConf{Default: "invalid"}).EffectiveMode(QualityProviderClaude))
	assert.Equal(t, "balanced", (QualityConf{Default: "invalid"}).EffectiveMode(QualityProviderCodex))
}

func TestQualityConfForProviderReturnsIsolatedEffectiveView(t *testing.T) {
	t.Parallel()

	quality := QualityConf{
		Default: "balanced",
		Providers: map[string]string{
			QualityProviderClaude: "ultra",
		},
	}

	effective := quality.ForProvider(QualityProviderClaude)

	assert.Equal(t, "ultra", effective.Default)
	assert.Nil(t, effective.Providers)
	assert.Equal(t, "balanced", quality.Default)
	assert.Equal(t, "ultra", quality.Providers[QualityProviderClaude])
}

func TestNormalizeQualityProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "claude", want: QualityProviderClaude, ok: true},
		{input: "claude-code", want: QualityProviderClaude, ok: true},
		{input: " CLAUDE-CODE ", want: QualityProviderClaude, ok: true},
		{input: "codex", want: QualityProviderCodex, ok: true},
		{input: "opencode", ok: false},
		{input: "", ok: false},
	}

	for _, tt := range tests {
		got, ok := NormalizeQualityProvider(tt.input)
		assert.Equal(t, tt.ok, ok, tt.input)
		assert.Equal(t, tt.want, got, tt.input)
	}
}

func TestHarnessConfigValidateQualityProviders(t *testing.T) {
	t.Parallel()

	t.Run("valid independent modes", func(t *testing.T) {
		cfg := DefaultFullConfig("provider-quality")
		cfg.Platforms = []string{"claude-code", "codex"}
		cfg.Quality.Providers = map[string]string{
			QualityProviderClaude: "ultra",
			QualityProviderCodex:  "balanced",
		}
		require.NoError(t, cfg.Validate())
	})

	t.Run("unknown provider", func(t *testing.T) {
		cfg := DefaultFullConfig("provider-quality")
		cfg.Quality.Providers = map[string]string{"claude-code": "ultra"}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), `quality.providers provider "claude-code"`)
	})

	t.Run("unknown preset", func(t *testing.T) {
		cfg := DefaultFullConfig("provider-quality")
		cfg.Quality.Providers = map[string]string{QualityProviderCodex: "turbo"}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), `quality.providers[codex] "turbo"`)
	})
}

func TestHarnessConfigRejectsUnsafeQualityPresetNames(t *testing.T) {
	t.Parallel()

	for _, preset := range []string{
		"custom # comment",
		"custom:value",
		"custom;command",
		"custom'quote",
		"custom\ninjected: value",
		"_leading",
		"-leading",
		strings.Repeat("a", 65),
	} {
		preset := preset
		t.Run(preset, func(t *testing.T) {
			cfg := DefaultFullConfig("unsafe-preset")
			cfg.Quality.Presets[preset] = cfg.Quality.Presets["balanced"]
			cfg.Quality.Providers = map[string]string{QualityProviderClaude: preset}

			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "quality.presets name")
		})
	}
}

func TestQualityPresetNameAllowsBoundedIdentifiers(t *testing.T) {
	t.Parallel()

	for _, preset := range []string{"balanced", "custom-safe", "custom_safe", "V2"} {
		assert.True(t, IsValidQualityPresetName(preset), preset)
	}
}

func TestCodexProfilesUseCodexProviderMode(t *testing.T) {
	t.Parallel()

	balancedGlobalCodexUltra := DefaultFullConfig("codex-ultra").Quality
	balancedGlobalCodexUltra.Providers = map[string]string{
		QualityProviderClaude: "balanced",
		QualityProviderCodex:  "ultra",
	}
	assert.Equal(t,
		CodexProfile{Model: CodexSolModel, Effort: CodexEffortUltra},
		balancedGlobalCodexUltra.CodexSupervisorProfile(),
	)
	assert.Equal(t,
		CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh},
		balancedGlobalCodexUltra.CodexAgentProfile("executor", "sonnet", "medium"),
	)
	assert.Equal(t,
		CodexProfile{Model: CodexSolModel, Effort: CodexEffortMax},
		balancedGlobalCodexUltra.CodexOrchestraProfile(),
	)
	assert.Equal(t,
		CodexProfile{Model: CodexSolModel, Effort: CodexEffortMax},
		balancedGlobalCodexUltra.CodexAgentProfile("planner", "opus", "medium"),
	)

	ultraGlobalCodexBalanced := DefaultFullConfig("codex-balanced").Quality
	ultraGlobalCodexBalanced.Default = "ultra"
	ultraGlobalCodexBalanced.Providers = map[string]string{
		QualityProviderClaude: "ultra",
		QualityProviderCodex:  "balanced",
	}
	assert.Equal(t,
		CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh},
		ultraGlobalCodexBalanced.CodexSupervisorProfile(),
	)
	assert.Equal(t,
		CodexProfile{Model: CodexTerraModel, Effort: CodexEffortMedium},
		ultraGlobalCodexBalanced.CodexAgentProfile("executor", "opus", "medium"),
	)
}
