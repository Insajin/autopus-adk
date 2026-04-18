package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMergeVerdictsSupermajorityPass: [PASS, PASS, REVISE] + threshold 0.67 → VerdictPass
func TestMergeVerdictsSupermajorityPass(t *testing.T) {
	t.Parallel()

	results := []ReviewResult{
		{Verdict: VerdictPass},
		{Verdict: VerdictPass},
		{Verdict: VerdictRevise},
	}
	// 2/3 = 0.667 >= 0.67 → PASS supermajority
	got := MergeVerdicts(results, 0.67, 3)
	assert.Equal(t, VerdictPass, got)
}

// TestMergeVerdictsSupermajorityRevise: [PASS, REVISE, REVISE] + threshold 0.67 → VerdictRevise
func TestMergeVerdictsSupermajorityRevise(t *testing.T) {
	t.Parallel()

	results := []ReviewResult{
		{Verdict: VerdictPass},
		{Verdict: VerdictRevise},
		{Verdict: VerdictRevise},
	}
	// 2/3 REVISE supermajority → REVISE
	got := MergeVerdicts(results, 0.67, 3)
	assert.Equal(t, VerdictRevise, got)
}

// TestMergeVerdictsRejectOverrides: [PASS, PASS, REJECT] → VerdictReject regardless of threshold
func TestMergeVerdictsRejectOverrides(t *testing.T) {
	t.Parallel()

	results := []ReviewResult{
		{Verdict: VerdictPass},
		{Verdict: VerdictPass},
		{Verdict: VerdictReject},
	}
	// 1 REJECT is a security gate — overrides supermajority
	got := MergeVerdicts(results, 0.67, 3)
	assert.Equal(t, VerdictReject, got)
}

// TestMergeVerdictsUnanimousThreshold: threshold 1.0, [PASS, PASS, REVISE] → VerdictRevise
// Backward-compat: with threshold=1.0, 2/3 PASS does not meet threshold → REVISE wins.
func TestMergeVerdictsUnanimousThreshold(t *testing.T) {
	t.Parallel()

	results := []ReviewResult{
		{Verdict: VerdictPass},
		{Verdict: VerdictPass},
		{Verdict: VerdictRevise},
	}
	// With threshold=1.0, PASS supermajority requires all 3 → fails → REVISE
	got := MergeVerdicts(results, 1.0, 3)
	assert.Equal(t, VerdictRevise, got)
}

// TestParseDiscoverFindingsEmptyID: all returned findings have ID == ""
// The scaffold (RED) test — parseDiscoverFindings currently assigns IDs internally.
// After REQ-05 lands, IDs are assigned globally by DeduplicateFindings, so
// parseDiscoverFindings must return findings with empty IDs.
func TestParseDiscoverFindingsEmptyID(t *testing.T) {
	t.Parallel()

	output := `VERDICT: REVISE
FINDING: [major] [correctness] pkg/spec/reviewer.go:54 seq restarts at 1 per provider
FINDING: [minor] [style] pkg/spec/types.go:12 inconsistent naming`

	findings := parseDiscoverFindings(output, "claude", 0)
	require.NotEmpty(t, findings)
	for _, f := range findings {
		assert.Equal(t, "", f.ID, "parseDiscoverFindings must not assign IDs; DeduplicateFindings owns ID assignment")
	}
}

// TestDeduplicateFindingsAssignsGlobalIDs: input with empty IDs → output has F-001, F-002...
// Verifies that DeduplicateFindings is the sole owner of global sequential ID assignment.
func TestDeduplicateFindingsAssignsGlobalIDs(t *testing.T) {
	t.Parallel()

	findings := []ReviewFinding{
		{Provider: "claude", Category: FindingCategoryCorrectness, ScopeRef: "pkg/spec/reviewer.go:54", Description: "seq restarts at 1 per provider"},
		{Provider: "gemini", Category: FindingCategoryStyle, ScopeRef: "pkg/spec/types.go:12", Description: "inconsistent naming"},
	}

	deduped := DeduplicateFindings(findings)
	require.Len(t, deduped, 2)
	assert.Equal(t, "F-001", deduped[0].ID)
	assert.Equal(t, "F-002", deduped[1].ID)
}
