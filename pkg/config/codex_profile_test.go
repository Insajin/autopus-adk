package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityConfCodexSupervisorProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		quality QualityConf
		want    CodexProfile
	}{
		{name: "balanced", quality: QualityConf{Default: "balanced"}, want: CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh}},
		{name: "ultra", quality: QualityConf{Default: "ultra"}, want: CodexProfile{Model: CodexSolModel, Effort: CodexEffortUltra}},
		{name: "invalid falls back to balanced", quality: QualityConf{Default: "unsupported"}, want: CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.quality.CodexSupervisorProfile())
			assert.Equal(t, tt.want.Model, tt.quality.CodexSupervisorModel())
			assert.Equal(t, tt.want.Effort, tt.quality.CodexSupervisorEffort())
		})
	}
}

func TestQualityConfCodexOrchestraProfile(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh},
		(QualityConf{Default: "balanced"}).CodexOrchestraProfile(),
	)
	assert.Equal(t,
		CodexProfile{Model: CodexSolModel, Effort: CodexEffortMax},
		(QualityConf{Default: "ultra"}).CodexOrchestraProfile(),
	)
}

func TestQualityConfCodexAgentProfile(t *testing.T) {
	t.Parallel()

	balanced := QualityConf{
		Default: "balanced",
		Presets: map[string]QualityPreset{
			"balanced": {Agents: map[string]string{
				"planner":  "opus",
				"executor": "sonnet",
				"explorer": "haiku",
			}},
		},
	}

	tests := []struct {
		name           string
		quality        QualityConf
		agent          string
		fallbackTier   string
		declaredEffort string
		want           CodexProfile
	}{
		{name: "balanced opus", quality: balanced, agent: "planner", fallbackTier: "sonnet", declaredEffort: "max", want: CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh}},
		{name: "balanced sonnet", quality: balanced, agent: "executor", fallbackTier: "opus", declaredEffort: "medium", want: CodexProfile{Model: CodexTerraModel, Effort: CodexEffortMedium}},
		{name: "balanced haiku", quality: balanced, agent: "explorer", fallbackTier: "opus", declaredEffort: "low", want: CodexProfile{Model: CodexLunaModel, Effort: CodexEffortLow}},
		{name: "fallback tier", quality: balanced, agent: "unmapped", fallbackTier: "haiku", declaredEffort: "high", want: CodexProfile{Model: CodexLunaModel, Effort: CodexEffortHigh}},
		{name: "invalid effort defaults medium", quality: balanced, agent: "executor", fallbackTier: "sonnet", declaredEffort: "invalid", want: CodexProfile{Model: CodexTerraModel, Effort: CodexEffortMedium}},
		{name: "balanced agents clamp declared ultra to max", quality: balanced, agent: "executor", fallbackTier: "sonnet", declaredEffort: "ultra", want: CodexProfile{Model: CodexTerraModel, Effort: CodexEffortMax}},
		{name: "ultra planner uses max", quality: QualityConf{Default: "ultra"}, agent: "planner", fallbackTier: "sonnet", declaredEffort: "medium", want: CodexProfile{Model: CodexSolModel, Effort: CodexEffortMax}},
		{name: "ultra architect uses max", quality: QualityConf{Default: "ultra"}, agent: "architect", fallbackTier: "sonnet", declaredEffort: "medium", want: CodexProfile{Model: CodexSolModel, Effort: CodexEffortMax}},
		{name: "ultra security auditor uses max", quality: QualityConf{Default: "ultra"}, agent: "security-auditor", fallbackTier: "sonnet", declaredEffort: "medium", want: CodexProfile{Model: CodexSolModel, Effort: CodexEffortMax}},
		{name: "ultra executor uses xhigh", quality: QualityConf{Default: "ultra"}, agent: "executor", fallbackTier: "opus", declaredEffort: "max", want: CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh}},
		{name: "ultra spec writer uses xhigh", quality: QualityConf{Default: "ultra"}, agent: "spec-writer", fallbackTier: "opus", declaredEffort: "max", want: CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh}},
		{name: "ultra underscore security name uses xhigh", quality: QualityConf{Default: "ultra"}, agent: "security_auditor", fallbackTier: "opus", declaredEffort: "max", want: CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh}},
		{name: "ultra unknown agent uses xhigh", quality: QualityConf{Default: "ultra"}, agent: "custom-agent", fallbackTier: "opus", declaredEffort: "max", want: CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh}},
		{name: "invalid quality follows balanced", quality: QualityConf{Default: "invalid"}, agent: "executor", fallbackTier: "sonnet", declaredEffort: "high", want: CodexProfile{Model: CodexTerraModel, Effort: CodexEffortHigh}},
		{name: "custom quality uses its role tier", quality: QualityConf{Default: "custom", Presets: map[string]QualityPreset{"custom": {Agents: map[string]string{"executor": "haiku"}}}}, agent: "executor", fallbackTier: "sonnet", declaredEffort: "high", want: CodexProfile{Model: CodexLunaModel, Effort: CodexEffortHigh}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.quality.CodexAgentProfile(tt.agent, tt.fallbackTier, tt.declaredEffort)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.want.Model, tt.quality.CodexAgentModel(tt.agent, tt.fallbackTier))
			assert.Equal(t, tt.want.Effort, tt.quality.CodexAgentEffort(tt.agent, tt.fallbackTier, tt.declaredEffort))
		})
	}
}

func TestQualityConfCodexUltraAgentProfileIgnoresTierAndDeclaredEffort(t *testing.T) {
	t.Parallel()

	roles := []struct {
		name   string
		effort string
	}{
		{name: "planner", effort: CodexEffortMax},
		{name: "architect", effort: CodexEffortMax},
		{name: "security-auditor", effort: CodexEffortMax},
		{name: "executor", effort: CodexEffortXHigh},
		{name: "custom-agent", effort: CodexEffortXHigh},
	}
	tiers := []string{"opus", "sonnet", "haiku", "unknown", ""}
	efforts := []string{CodexEffortLow, CodexEffortMedium, CodexEffortHigh, CodexEffortXHigh, CodexEffortMax, CodexEffortUltra, "unknown", ""}
	ultra := QualityConf{Default: "ultra"}

	for _, role := range roles {
		want := CodexProfile{Model: CodexSolModel, Effort: role.effort}
		for _, tier := range tiers {
			for _, effort := range efforts {
				assert.Equal(t, want, ultra.CodexAgentProfile(role.name, tier, effort),
					"role=%s tier=%q declared_effort=%q", role.name, tier, effort)
			}
		}
	}
}

func TestParseCodexModelCatalog(t *testing.T) {
	t.Parallel()

	catalog, err := ParseCodexModelCatalog([]byte(`{
		"models": [
			{
				"slug": "gpt-5.6-sol",
				"default_reasoning_level": "low",
				"supported_reasoning_levels": [
					{"effort": "xhigh", "description": "deep"},
					{"effort": "max", "description": "deeper"},
					{"effort": "ultra", "description": "delegated"}
				]
			}
		]
	}`))
	require.NoError(t, err)
	require.Len(t, catalog.Models, 1)
	assert.Equal(t, CodexSolModel, catalog.Models[0].Slug)
	assert.Equal(t, CodexEffortLow, catalog.Models[0].DefaultReasoningLevel)
	assert.True(t, catalog.Supports(CodexSolModel, CodexEffortUltra))
	assert.False(t, catalog.Supports(CodexSolModel, "unsupported"))
	assert.False(t, catalog.Supports("missing", CodexEffortXHigh))
}

func TestParseCodexModelCatalogRejectsInvalidOrEmptyCatalog(t *testing.T) {
	t.Parallel()

	for _, input := range [][]byte{nil, []byte(`not-json`), []byte(`{}`), []byte(`{"models": []}`)} {
		_, err := ParseCodexModelCatalog(input)
		require.Error(t, err)
	}
}

func TestResolveCodexProfile(t *testing.T) {
	t.Parallel()

	catalog := []byte(`{
		"models": [
			{"slug":"gpt-5.6-sol","supported_reasoning_levels":[{"effort":"xhigh"},{"effort":"max"},{"effort":"ultra"}]},
			{"slug":"gpt-5.6-luna","supported_reasoning_levels":[{"effort":"xhigh"},{"effort":"max"}]},
			{"slug":"gpt-5.5","supported_reasoning_levels":[{"effort":"xhigh"}]}
		]
	}`)

	t.Run("supported profile remains unchanged", func(t *testing.T) {
		got := ResolveCodexProfile(CodexProfile{Model: CodexSolModel, Effort: CodexEffortUltra}, catalog)
		assert.Equal(t, CodexResolutionSupported, got.Reason)
		assert.False(t, got.Fallback)
		assert.Equal(t, got.Requested, got.Effective)
	})

	t.Run("unsupported effort uses highest supported effort on same model", func(t *testing.T) {
		got := ResolveCodexProfile(CodexProfile{Model: CodexLunaModel, Effort: CodexEffortUltra}, catalog)
		assert.Equal(t, CodexResolutionEffortUnavailable, got.Reason)
		assert.True(t, got.Fallback)
		assert.Equal(t, CodexProfile{Model: CodexLunaModel, Effort: CodexEffortMax}, got.Effective)
	})

	t.Run("target model with no lower effort keeps model and defers effort", func(t *testing.T) {
		onlyHigherEffort := []byte(`{"models":[{"slug":"gpt-5.6-sol","supported_reasoning_levels":[{"effort":"medium"}]}]}`)
		got := ResolveCodexProfile(CodexProfile{Model: CodexSolModel, Effort: CodexEffortLow}, onlyHigherEffort)
		assert.Equal(t, CodexResolutionRuntimeDefault, got.Reason)
		assert.True(t, got.Fallback)
		assert.Equal(t, CodexProfile{Model: CodexSolModel}, got.Effective)
	})

	t.Run("unavailable model falls back to legacy and downgrades max", func(t *testing.T) {
		requested := CodexProfile{Model: CodexTerraModel, Effort: CodexEffortMax}
		got := ResolveCodexProfile(requested, catalog)
		assert.Equal(t, CodexResolutionModelUnavailable, got.Reason)
		assert.True(t, got.Fallback)
		assert.Equal(t, requested, got.Requested)
		assert.Equal(t, CodexProfile{Model: CodexLegacyModel, Effort: CodexEffortXHigh}, got.Effective)
	})

	t.Run("legacy model remains capped when catalog advertises newer efforts", func(t *testing.T) {
		legacyWithNewerEfforts := []byte(`{
			"models": [
				{"slug":"gpt-5.5","supported_reasoning_levels":[
					{"effort":"xhigh"},{"effort":"max"},{"effort":"ultra"}
				]}
			]
		}`)
		got := ResolveCodexProfile(
			CodexProfile{Model: CodexSolModel, Effort: CodexEffortUltra},
			legacyWithNewerEfforts,
		)
		assert.Equal(t, CodexResolutionModelUnavailable, got.Reason)
		assert.True(t, got.Fallback)
		assert.Equal(t, CodexProfile{Model: CodexLegacyModel, Effort: CodexEffortXHigh}, got.Effective)

		direct := ResolveCodexProfile(
			CodexProfile{Model: CodexLegacyModel, Effort: CodexEffortUltra},
			legacyWithNewerEfforts,
		)
		assert.Equal(t, CodexResolutionEffortUnavailable, direct.Reason)
		assert.True(t, direct.Fallback)
		assert.Equal(t, CodexProfile{Model: CodexLegacyModel, Effort: CodexEffortXHigh}, direct.Effective)
	})

	t.Run("missing target and legacy defers to runtime default", func(t *testing.T) {
		withoutLegacy := []byte(`{"models":[{"slug":"gpt-5.6-luna","supported_reasoning_levels":[{"effort":"medium"}]}]}`)
		got := ResolveCodexProfile(CodexProfile{Model: CodexTerraModel, Effort: CodexEffortMax}, withoutLegacy)
		assert.Equal(t, CodexResolutionRuntimeDefault, got.Reason)
		assert.True(t, got.Fallback)
		assert.Equal(t, CodexProfile{}, got.Effective)
	})
}

func TestResolveCodexProfileCatalogUnknown(t *testing.T) {
	t.Parallel()

	requested := CodexProfile{Model: CodexSolModel, Effort: CodexEffortUltra}
	for _, catalog := range [][]byte{nil, []byte(`invalid`), []byte(`{"models":[]}`)} {
		got := ResolveCodexProfile(requested, catalog)
		assert.Equal(t, CodexResolutionCatalogUnknown, got.Reason)
		assert.True(t, got.Fallback)
		assert.Equal(t, requested, got.Requested)
		assert.Equal(t, CodexProfile{Model: CodexLegacyModel, Effort: CodexEffortXHigh}, got.Effective)
		assert.Error(t, got.CatalogError)
	}
}

func TestCodexProviderEntryForQuality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		quality    QualityConf
		wantModel  string
		wantEffort string
	}{
		{name: "balanced", quality: QualityConf{Default: "balanced"}, wantModel: CodexSolModel, wantEffort: CodexEffortXHigh},
		{name: "ultra", quality: QualityConf{Default: "ultra"}, wantModel: CodexSolModel, wantEffort: CodexEffortMax},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry := CodexProviderEntryForQuality(tt.quality)
			assert.Equal(t, []string{"exec", "--sandbox", "workspace-write", "-m", tt.wantModel, "-c", `model_reasoning_effort="` + tt.wantEffort + `"`}, entry.Args)
			assert.Equal(t, []string{"-m", tt.wantModel, "-c", `model_reasoning_effort="` + tt.wantEffort + `"`}, entry.PaneArgs)
		})
	}
}
