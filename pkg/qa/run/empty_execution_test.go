package run

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A lane no Journey Pack declares executes nothing. Once its own setup gaps are
// satisfied, aggregateStatus sees no failure and no gap, so without this guard it
// returns "passed" over an empty result - a green lane backed by no evidence.
// Observed on `auto qa run --lane mobile-readiness` after a complete
// .autopus/qa/mobile/readiness.yaml removed every readiness gap: status passed,
// zero adapters, zero checks, zero manifests.
func TestNoteEmptyExecutionRefusesSilentPass(t *testing.T) {
	t.Parallel()

	result := Result{
		SetupGaps:      []SetupGap{},
		AdapterResults: []AdapterResult{},
		Checks:         []IndexCheck{},
	}
	noteEmptyExecution(&result, "mobile-readiness")

	require.Len(t, result.SetupGaps, 1)
	assert.Equal(t, "harness", result.SetupGaps[0].Adapter)
	assert.Contains(t, result.SetupGaps[0].Reason, "missing_journey_pack")
	assert.Contains(t, result.SetupGaps[0].Reason, `"mobile-readiness"`)
	assert.Contains(t, result.SetupGaps[0].Reason, "nothing was executed")
	// The status must follow from the existing gap rule rather than a second one.
	assert.Equal(t, "warning", aggregateStatus(result))
}

// The guard must not fire when work actually happened, or every real run would
// grow a phantom gap.
func TestNoteEmptyExecutionLeavesRealRunsAlone(t *testing.T) {
	t.Parallel()

	cases := map[string]Result{
		"adapter ran": {
			AdapterResults: []AdapterResult{{Adapter: "go-test", Status: "passed"}},
			Checks:         []IndexCheck{},
			SetupGaps:      []SetupGap{},
		},
		"check produced": {
			AdapterResults: []AdapterResult{},
			Checks:         []IndexCheck{{ID: "go-fast", Status: "passed"}},
			SetupGaps:      []SetupGap{},
		},
		"adapter skipped by profile": {
			AdapterResults: []AdapterResult{{Adapter: "playwright", Status: "skipped"}},
			Checks:         []IndexCheck{},
			SetupGaps:      []SetupGap{},
		},
	}
	for name, base := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result := base
			noteEmptyExecution(&result, "fast")
			assert.Empty(t, result.SetupGaps)
			assert.Equal(t, "passed", aggregateStatus(result))
		})
	}
}

// An existing gap already explains the empty run; appending a second one would
// duplicate the diagnosis and hide the more specific reason.
func TestNoteEmptyExecutionPreservesExistingGap(t *testing.T) {
	t.Parallel()

	result := Result{
		SetupGaps:      []SetupGap{{Adapter: "mobile-readiness", Reason: "missing_device_inventory: device inventory is required"}},
		AdapterResults: []AdapterResult{},
		Checks:         []IndexCheck{},
	}
	noteEmptyExecution(&result, "mobile-readiness")

	require.Len(t, result.SetupGaps, 1)
	assert.Contains(t, result.SetupGaps[0].Reason, "missing_device_inventory")
}

// An empty lane name must still produce a readable reason rather than an
// unquoted blank that reads as a truncated message.
func TestNoteEmptyExecutionHandlesBlankLane(t *testing.T) {
	t.Parallel()

	result := Result{SetupGaps: []SetupGap{}, AdapterResults: []AdapterResult{}, Checks: []IndexCheck{}}
	noteEmptyExecution(&result, "   ")

	require.Len(t, result.SetupGaps, 1)
	assert.Contains(t, result.SetupGaps[0].Reason, "the selected lane")
	assert.False(t, strings.Contains(result.SetupGaps[0].Reason, `""`))
}
