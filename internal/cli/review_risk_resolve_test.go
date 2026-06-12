package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseReviewRiskTier_Valid covers each accepted tier alias.
func TestParseReviewRiskTier_Valid(t *testing.T) {
	t.Parallel()

	cases := map[string]reviewRiskTier{
		"":         reviewRiskTierAuto,
		"auto":     reviewRiskTierAuto,
		" AUTO ":   reviewRiskTierAuto,
		"low":      reviewRiskTierLow,
		"Medium":   reviewRiskTierMedium,
		"HIGH":     reviewRiskTierHigh,
		"critical": reviewRiskTierCritical,
	}
	for in, want := range cases {
		got, err := parseReviewRiskTier(in)
		require.NoError(t, err, "input %q", in)
		assert.Equal(t, want, got, "input %q", in)
	}
}

// TestParseReviewRiskTier_Invalid returns an explanatory error.
func TestParseReviewRiskTier_Invalid(t *testing.T) {
	t.Parallel()

	got, err := parseReviewRiskTier("extreme")
	require.Error(t, err)
	assert.Empty(t, string(got))
	assert.Contains(t, err.Error(), "invalid risk tier")
}

// TestResolveReviewRiskTier_ExplicitTier short-circuits inference and git lookup.
func TestResolveReviewRiskTier_ExplicitTier(t *testing.T) {
	t.Parallel()

	tier, inputs, err := resolveReviewRiskTier("high", []string{"./internal/services/x.go", " "})
	require.NoError(t, err)
	assert.Equal(t, reviewRiskTierHigh, tier)
	// Normalization strips the ./ prefix and blank entries.
	require.Len(t, inputs, 1)
	assert.Equal(t, "internal/services/x.go", inputs[0])
}

// TestResolveReviewRiskTier_AutoWithFiles infers a tier from the provided files.
func TestResolveReviewRiskTier_AutoWithFiles(t *testing.T) {
	t.Parallel()

	tier, inputs, err := resolveReviewRiskTier("auto", []string{"docs/readme.md", "docs/guide.md"})
	require.NoError(t, err)
	assert.Equal(t, reviewRiskTierLow, tier)
	assert.Len(t, inputs, 2)
}

// TestResolveReviewRiskTier_AutoCriticalPath escalates to critical on sensitive paths.
func TestResolveReviewRiskTier_AutoCriticalPath(t *testing.T) {
	t.Parallel()

	tier, _, err := resolveReviewRiskTier("auto", []string{"pkg/auth/token.go"})
	require.NoError(t, err)
	assert.Equal(t, reviewRiskTierCritical, tier)
}

// TestResolveReviewRiskTier_InvalidPropagatesError surfaces parse errors.
func TestResolveReviewRiskTier_InvalidPropagatesError(t *testing.T) {
	t.Parallel()

	_, _, err := resolveReviewRiskTier("nope", nil)
	require.Error(t, err)
}

// TestInferReviewRiskTier_SourceThresholds covers high-by-count and medium paths.
func TestInferReviewRiskTier_SourceThresholds(t *testing.T) {
	t.Parallel()

	// Five source files force a high tier without any critical/high path token.
	manySources := []string{"a.go", "b.go", "c.ts", "d.rs", "e.py"}
	assert.Equal(t, reviewRiskTierHigh, inferReviewRiskTier(manySources))

	// One ordinary source file maps to medium.
	assert.Equal(t, reviewRiskTierMedium, inferReviewRiskTier([]string{"a.go"}))

	// Non-source, non-doc files map to low.
	assert.Equal(t, reviewRiskTierLow, inferReviewRiskTier([]string{"data.csv"}))

	// Empty input defaults to medium.
	assert.Equal(t, reviewRiskTierMedium, inferReviewRiskTier(nil))
}

// TestNormalizeRiskTierFiles_TrimsAndDrops removes blanks and prefixes.
func TestNormalizeRiskTierFiles_TrimsAndDrops(t *testing.T) {
	t.Parallel()

	got := normalizeRiskTierFiles([]string{"./x.go", "", ".", "  y.ts  ", "."})
	assert.Equal(t, []string{"x.go", "y.ts"}, got)
}

// TestTierRequiresMultipleProviders flags only high and critical tiers.
func TestTierRequiresMultipleProviders(t *testing.T) {
	t.Parallel()

	assert.True(t, tierRequiresMultipleProviders(reviewRiskTierHigh))
	assert.True(t, tierRequiresMultipleProviders(reviewRiskTierCritical))
	assert.False(t, tierRequiresMultipleProviders(reviewRiskTierLow))
	assert.False(t, tierRequiresMultipleProviders(reviewRiskTierMedium))
}
