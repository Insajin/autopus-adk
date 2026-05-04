package spec

// Phase 1.5 test scaffold for SPEC-SPECREV-001 REQ-VERD-3.
// References MergeVerdictsWithDenomMode which does not yet exist —
// compile failure is the expected RED state.

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMergeVerdictsWithDenomMode_BackCompatDelegation covers
// AC-VERD-BACKCOMPAT: excludeFailed=false MUST mirror legacy MergeVerdicts.
func TestMergeVerdictsWithDenomMode_BackCompatDelegation(t *testing.T) {
	t.Parallel()

	results := []ReviewResult{
		{Verdict: VerdictPass},
		{Verdict: VerdictPass},
		{Verdict: VerdictRevise},
	}

	legacy := MergeVerdicts(results, 0.67, 3)
	got := MergeVerdictsWithDenomMode(results, 0.67, 3, false, 0)

	assert.Equal(t, legacy, got, "excludeFailed=false must equal MergeVerdicts(legacy)")
	assert.Equal(t, VerdictRevise, got, "1 REVISE keeps verdict at REVISE")
}

// TestMergeVerdictsWithDenomMode_ExcludeFailedSurvivorPass covers AC-VERD-3:
// 1 PASS / 2 timeout with exclude=true => denom=1, passCount=1 => PASS.
func TestMergeVerdictsWithDenomMode_ExcludeFailedSurvivorPass(t *testing.T) {
	t.Parallel()

	// Only the surviving (non-failed) results are in `results`.
	results := []ReviewResult{
		{Verdict: VerdictPass},
	}

	got := MergeVerdictsWithDenomMode(results, 0.67, 3, true, 2)

	assert.Equal(t, VerdictPass, got,
		"with excludeFailed=true and 1 survivor PASS, denom=3-2=1 -> PASS")
}

// TestMergeVerdictsWithDenomMode_ExcludeFailedAllFailed covers AC-VERD-EMPTY:
// when all providers failed, denom=0 must yield VerdictRevise (NOT VerdictReject).
func TestMergeVerdictsWithDenomMode_ExcludeFailedAllFailed(t *testing.T) {
	t.Parallel()

	results := []ReviewResult{}

	got := MergeVerdictsWithDenomMode(results, 0.67, 3, true, 3)

	assert.Equal(t, VerdictRevise, got,
		"all providers failed with excludeFailed=true must return VerdictRevise (not Reject)")
}

// TestMergeVerdictsWithDenomMode_TableDriven aggregates further denom-mode cases.
func TestMergeVerdictsWithDenomMode_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		results          []ReviewResult
		threshold        float64
		totalProviders   int
		excludeFailed    bool
		failedCount      int
		want             ReviewVerdict
	}{
		{
			name: "exclude=false 2 PASS / 1 REVISE -> REVISE",
			results: []ReviewResult{
				{Verdict: VerdictPass},
				{Verdict: VerdictPass},
				{Verdict: VerdictRevise},
			},
			threshold: 0.67, totalProviders: 3, excludeFailed: false, failedCount: 0,
			want: VerdictRevise,
		},
		{
			name: "exclude=true 2 PASS surviving / 1 timeout -> PASS",
			results: []ReviewResult{
				{Verdict: VerdictPass},
				{Verdict: VerdictPass},
			},
			threshold: 0.67, totalProviders: 3, excludeFailed: true, failedCount: 1,
			want: VerdictPass,
		},
		{
			name: "REJECT short-circuits regardless of denom mode",
			results: []ReviewResult{
				{Verdict: VerdictReject},
			},
			threshold: 0.67, totalProviders: 3, excludeFailed: true, failedCount: 2,
			want: VerdictReject,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MergeVerdictsWithDenomMode(tt.results, tt.threshold, tt.totalProviders, tt.excludeFailed, tt.failedCount)
			assert.Equal(t, tt.want, got)
		})
	}
}
