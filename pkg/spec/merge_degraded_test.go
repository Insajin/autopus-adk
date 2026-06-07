package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// When excludeFailed=true but the surviving results fail to reach the
// supermajority over the failure-adjusted denominator, the merge must REVISE
// rather than silently PASS on a diluted survivor. This gives the excludeFailed
// branch the same AC-VERD-1 protection the legacy branch already has.
func TestMergeVerdictsWithDenomMode_ExcludeFailedDilutedSurvivorRevises(t *testing.T) {
	t.Parallel()

	// 3 configured, 1 counted as failed -> denom=2; a lone PASS is 1/2 < 0.67.
	results := []ReviewResult{{Verdict: VerdictPass}}
	got := MergeVerdictsWithDenomMode(results, 0.67, 3, true, 1)

	assert.Equal(t, VerdictRevise, got,
		"diluted single survivor below supermajority must REVISE, not silently PASS")
}

// Guard against over-correction: when the denominator legitimately collapses to
// 1 (all other providers failed), a single PASS survivor still PASSes.
func TestMergeVerdictsWithDenomMode_ExcludeFailedTrueSurvivorStillPasses(t *testing.T) {
	t.Parallel()

	results := []ReviewResult{{Verdict: VerdictPass}}
	got := MergeVerdictsWithDenomMode(results, 0.67, 3, true, 2)

	assert.Equal(t, VerdictPass, got,
		"denom=1 single survivor PASS must remain PASS (degraded but conclusive)")
}
