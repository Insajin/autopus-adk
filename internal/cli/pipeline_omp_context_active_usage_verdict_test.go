package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func usage(input, output, total int64) pipelineOMPActiveUsage {
	return pipelineOMPActiveUsage{Input: input, Output: output, Total: total}
}

// A call that moved every axis is the only accepted shape.
func TestPipelineOMPActiveUsageVerdict_AcceptsRealCall(t *testing.T) {
	t.Parallel()
	require.NoError(t, pipelineOMPActiveUsageVerdict(
		usage(1000, 10, 1010), usage(1500, 22, 1522), 320000))
}

// The failure that stalled the v0.50.104 cohort at call 13: accumulation reached
// the declared window, OMP stopped calling the provider, and every axis went
// flat. The message has to name that cause, because the alternative is bisecting
// a 40-call production run.
func TestPipelineOMPActiveUsageVerdict_NamesDeclaredWindowExhaustion(t *testing.T) {
	t.Parallel()
	err := pipelineOMPActiveUsageVerdict(
		usage(1130394, 24, 1130418), usage(1130394, 24, 1130418), 320000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cumulative input 1130394")
	assert.Contains(t, err.Error(), "declared model context window 320000")
	assert.Contains(t, err.Error(), "does not match the model's real")
}

// Below the declared window the exhaustion explanation would be wrong, so the
// generic form must report the raw observation instead of guessing.
func TestPipelineOMPActiveUsageVerdict_ReportsRawObservationBelowWindow(t *testing.T) {
	t.Parallel()
	err := pipelineOMPActiveUsageVerdict(
		usage(500, 10, 510), usage(500, 10, 510), 320000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delta=0/0/0")
	assert.Contains(t, err.Error(), "before=500/10/510")
	assert.Contains(t, err.Error(), "after=500/10/510")
	assert.NotContains(t, err.Error(), "declared model context window")
}

// An unasserted window must not produce an exhaustion claim, however large the
// cumulative input is.
func TestPipelineOMPActiveUsageVerdict_UnknownWindowStaysGeneric(t *testing.T) {
	t.Parallel()
	err := pipelineOMPActiveUsageVerdict(
		usage(9_000_000, 24, 9_000_024), usage(9_000_000, 24, 9_000_024), 0)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "declared model context window")
}

// Each axis is independently load-bearing: a call that produced tokens on only
// one side, or a total that cannot cover its own components, is still rejected.
func TestPipelineOMPActiveUsageVerdict_RejectsEachFlatAxis(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ before, after pipelineOMPActiveUsage }{
		"input flat":      {usage(100, 10, 110), usage(100, 20, 120)},
		"output flat":     {usage(100, 10, 110), usage(200, 10, 210)},
		"total shortfall": {usage(100, 10, 110), usage(200, 20, 215)},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, pipelineOMPActiveUsageVerdict(tc.before, tc.after, 320000))
		})
	}
}
