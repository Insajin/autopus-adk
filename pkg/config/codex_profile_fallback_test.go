package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A ChatGPT account that is not entitled to the frontier model still lists the
// current-generation balanced model. Resolving to the legacy slug there turns a
// top-tier request into a generation downgrade, which is worse than the mid
// tier the caller would have gotten without asking for the top tier at all.
func TestResolveCodexProfile_FrontierMissingPrefersSameGeneration(t *testing.T) {
	t.Parallel()
	// Shape of a real `codex debug models` payload without gpt-5.6-sol.
	catalog := []byte(`{"models":[
		{"slug":"gpt-5.6-terra","supported_reasoning_levels":[{"effort":"medium"},{"effort":"high"},{"effort":"xhigh"},{"effort":"max"}]},
		{"slug":"gpt-5.6-luna","supported_reasoning_levels":[{"effort":"medium"},{"effort":"xhigh"}]},
		{"slug":"gpt-5.5","supported_reasoning_levels":[{"effort":"xhigh"}]}
	]}`)

	got := ResolveCodexProfile(CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh}, catalog)

	assert.Equal(t, CodexResolutionModelUnavailable, got.Reason)
	assert.True(t, got.Fallback)
	assert.Equal(t, CodexProfile{Model: CodexTerraModel, Effort: CodexEffortXHigh}, got.Effective)
}

// With no same-generation substitute present, the legacy slug is still the
// answer — the new step must not strand callers on older catalogs.
func TestResolveCodexProfile_FrontierMissingFallsBackToLegacyWithoutMidTier(t *testing.T) {
	t.Parallel()
	catalog := []byte(`{"models":[{"slug":"gpt-5.5","supported_reasoning_levels":[{"effort":"xhigh"}]}]}`)

	got := ResolveCodexProfile(CodexProfile{Model: CodexSolModel, Effort: CodexEffortUltra}, catalog)

	assert.Equal(t, CodexResolutionModelUnavailable, got.Reason)
	assert.Equal(t, CodexProfile{Model: CodexLegacyModel, Effort: CodexEffortXHigh}, got.Effective)
}

// The small tier is the explicitly cheap model, so it is never promoted into a
// substitute for a larger one: a missing balanced tier still lands on legacy
// even when the small tier is available.
func TestResolveCodexProfile_MidTierMissingSkipsSmallTier(t *testing.T) {
	t.Parallel()
	catalog := []byte(`{"models":[
		{"slug":"gpt-5.6-luna","supported_reasoning_levels":[{"effort":"medium"},{"effort":"xhigh"},{"effort":"max"}]},
		{"slug":"gpt-5.5","supported_reasoning_levels":[{"effort":"xhigh"}]}
	]}`)

	got := ResolveCodexProfile(CodexProfile{Model: CodexTerraModel, Effort: CodexEffortMax}, catalog)

	assert.Equal(t, CodexResolutionModelUnavailable, got.Reason)
	assert.Equal(t, CodexProfile{Model: CodexLegacyModel, Effort: CodexEffortXHigh}, got.Effective)
}

// Nothing recognisable in the catalog must still defer to the runtime default
// rather than inventing a model the account cannot call.
func TestResolveCodexProfile_NoKnownSubstituteDefersToRuntime(t *testing.T) {
	t.Parallel()
	catalog := []byte(`{"models":[{"slug":"other-model","supported_reasoning_levels":[{"effort":"medium"}]}]}`)

	got := ResolveCodexProfile(CodexProfile{Model: CodexSolModel, Effort: CodexEffortXHigh}, catalog)

	assert.Equal(t, CodexResolutionRuntimeDefault, got.Reason)
	assert.True(t, got.Fallback)
	assert.Empty(t, got.Effective.Model)
}
