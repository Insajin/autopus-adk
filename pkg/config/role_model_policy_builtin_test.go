package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wantCandidate is the single derived candidate expected for one capability.
type wantCandidate struct {
	capability string
	selector   string
	thinking   string
}

func TestBuiltinRoleModelProfile_BalancedProjectsPresetTiers(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("builtin-balanced")
	cfg.RoleModelPolicy = RoleModelPolicyConf{Version: RoleModelPolicyVersionV1, Profile: "balanced"}
	require.NoError(t, cfg.Validate(), "selecting a built-in profile needs no explicit definition")

	name, profile, ok := cfg.RoleModelPolicy.SelectedRoleModelProfileForQuality(cfg.Quality)
	require.True(t, ok)
	assert.Equal(t, "balanced", name)
	assert.Equal(t, RoleModelConfigModeOverlay, profile.ConfigMode)
	assert.True(t, profile.FamilyDiversity.Enabled, "the OMP adapter requires an explicit diversity policy")
	require.NoError(t, validateRoleModelProfile(name, profile), "a derived profile must satisfy hand-written profile rules")

	// Capability tiers are the highest tier among the agents routed to them:
	// deep_reasoning (architect, planner, spec-writer) and coding_tool_use
	// (deep-worker, executor) are opus in the balanced preset, and
	// independent_dissent inherits opus from security-auditor.
	assertBuiltinCandidates(t, profile, []wantCandidate{
		{CapabilityDeepReasoning, "anthropic/" + ClaudeOpusModel, "xhigh"},
		{CapabilityCodingToolUse, "anthropic/" + ClaudeOpusModel, "xhigh"},
		{CapabilityIndependentDissent, "anthropic/" + ClaudeOpusModel, "xhigh"},
		{CapabilityFastValidation, "anthropic/" + ClaudeSonnetModel, "medium"},
		{CapabilityVisionDesign, "anthropic/" + ClaudeSonnetModel, "medium"},
		{CapabilityDeterministicTransform, "anthropic/" + ClaudeSonnetModel, "medium"},
	})
}

func TestBuiltinRoleModelProfile_UltraRunsEveryCapabilityAtTopTier(t *testing.T) {
	t.Parallel()

	quality := DefaultFullConfig("builtin-ultra").Quality
	profile, ok := BuiltinRoleModelProfile("ultra", quality)
	require.True(t, ok)
	require.NoError(t, validateRoleModelProfile("ultra", profile))

	for _, capability := range OMPProviderNeutralCapabilities() {
		route := profile.Capabilities[capability]
		require.Len(t, route.Candidates, 1, capability)
		assert.Equal(t, "anthropic/"+ClaudeOpusModel, route.Candidates[0].Selector, capability)
		assert.Equal(t, "xhigh", route.Candidates[0].Thinking, capability)
	}
}

func TestBuiltinRoleModelProfile_TakesHighestTierPerCapability(t *testing.T) {
	t.Parallel()

	quality := QualityConf{
		Default: "balanced",
		Presets: map[string]QualityPreset{"balanced": {Agents: map[string]string{
			// fast_validation and deterministic_transform have only haiku agents.
			"annotator": "haiku", "explorer": "haiku", "validator": "haiku",
			// coding_tool_use mixes opus with agents the preset omits, which
			// resolve to the balanced fallback tier: the top tier must win.
			"executor": "opus",
		}}},
	}

	profile, ok := BuiltinRoleModelProfile("balanced", quality)
	require.True(t, ok)
	assertBuiltinCandidates(t, profile, []wantCandidate{
		{CapabilityFastValidation, "anthropic/" + ClaudeHaikuModel, "low"},
		{CapabilityDeterministicTransform, "anthropic/" + ClaudeHaikuModel, "low"},
		{CapabilityCodingToolUse, "anthropic/" + ClaudeOpusModel, "xhigh"},
		{CapabilityDeepReasoning, "anthropic/" + ClaudeSonnetModel, "medium"},
		{CapabilityVisionDesign, "anthropic/" + ClaudeSonnetModel, "medium"},
		{CapabilityIndependentDissent, "anthropic/" + ClaudeSonnetModel, "medium"},
	})
}

func TestBuiltinRoleModelProfile_UsesModeTierWhenPresetIsAbsent(t *testing.T) {
	t.Parallel()

	// Only a balanced preset exists. The ultra profile must not borrow it, and
	// the balanced profile of a preset-less config must not borrow ultra.
	onlyBalanced := QualityConf{Presets: map[string]QualityPreset{
		"balanced": {Agents: map[string]string{"planner": "sonnet", "executor": "sonnet"}},
	}}
	onlyUltra := QualityConf{Presets: map[string]QualityPreset{
		"ultra": {Agents: map[string]string{"planner": "opus", "executor": "opus"}},
	}}

	ultra, ok := BuiltinRoleModelProfile("ultra", onlyBalanced)
	require.True(t, ok)
	balanced, ok := BuiltinRoleModelProfile("balanced", onlyUltra)
	require.True(t, ok)

	for _, capability := range OMPProviderNeutralCapabilities() {
		assert.Equal(t, "anthropic/"+ClaudeOpusModel, ultra.Capabilities[capability].Candidates[0].Selector, capability)
		assert.Equal(t, "anthropic/"+ClaudeSonnetModel, balanced.Capabilities[capability].Candidates[0].Selector, capability)
	}
}

func TestBuiltinRoleModelProfile_ExplicitDefinitionWins(t *testing.T) {
	t.Parallel()

	explicit := validRoleModelPolicyFixture().Profiles["p1"]
	policy := RoleModelPolicyConf{
		Version:  RoleModelPolicyVersionV1,
		Profile:  "balanced",
		Profiles: map[string]RoleModelProfileConf{"balanced": explicit},
	}

	name, profile, ok := policy.SelectedRoleModelProfileForQuality(DefaultFullConfig("explicit").Quality)
	require.True(t, ok)
	assert.Equal(t, "balanced", name)
	for _, capability := range OMPProviderNeutralCapabilities() {
		assert.Equal(t, "acme/model", profile.Capabilities[capability].Candidates[0].Selector, capability)
	}
}

func TestBuiltinRoleModelProfile_StaysOptIn(t *testing.T) {
	t.Parallel()

	quality := DefaultFullConfig("opt-in").Quality
	name, profile, ok := (RoleModelPolicyConf{}).SelectedRoleModelProfileForQuality(quality)
	assert.False(t, ok, "an unselected policy must not gain a derived profile")
	assert.Empty(t, name)
	assert.Empty(t, profile.Capabilities)

	unknown := RoleModelPolicyConf{Version: RoleModelPolicyVersionV1, Profile: "custom"}
	_, _, ok = unknown.SelectedRoleModelProfileForQuality(quality)
	assert.False(t, ok, "only built-in names are derived")
	assert.Error(t, unknown.Validate(), "an undefined non-built-in profile still fails closed")
}

func assertBuiltinCandidates(t *testing.T, profile RoleModelProfileConf, wants []wantCandidate) {
	t.Helper()
	require.Len(t, profile.Capabilities, len(OMPProviderNeutralCapabilities()))
	for _, want := range wants {
		route, ok := profile.Capabilities[want.capability]
		require.True(t, ok, want.capability)
		require.Len(t, route.Candidates, 1, want.capability)
		assert.Equal(t, want.selector, route.Candidates[0].Selector, want.capability)
		assert.Equal(t, want.thinking, route.Candidates[0].Thinking, want.capability)
		assert.Equal(t, "runtime_default", route.DegradedAction, want.capability)
	}
}
