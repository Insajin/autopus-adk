package cli

// AC-013 on the real protocol, without a child process. These two cases are
// the ones the cohort cannot show precisely: the exact position of the
// canonical re-admission inside the transaction, and a compaction that is
// declined after the repeated pre-checkpoint was already acknowledged.

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// S13 end to end on one transaction: p1 and p2 acknowledged once each, the
// canonical prompt re-admitted exactly at q, the transcript untouched until
// the transaction settles.
func TestPipelineOMPActiveManualCompactCompletesRepeatedPreCheckpoint(t *testing.T) {
	exchange := runPipelineOMPActiveCheckpointExchange(t, pipelineOMPActiveCheckpointRepeat)

	require.NoError(t, exchange.err)
	assert.True(t, exchange.performed)
	assert.Equal(t, workflowContextCheckpointFirstACKs("pre-1", "pre-2", "post"), exchange.acks)
	assert.Equal(t, 1, exchange.readmissions, "canonical re-admission runs once")
	assert.Equal(t, 2, exchange.readmissionAt,
		"it runs at the post-checkpoint, after both pre-checkpoints and before the post is answered")
	assert.Equal(t, 1, exchange.loopback.compactions)
	assert.Len(t, exchange.loopback.transcript, 2, "the summary lands only after the transaction settles")

	record := exchange.record
	assert.Equal(t, ompprobe.OutcomeCompleted, record.Outcome)
	assert.Equal(t, ompprobe.MethodSnapcompact, record.Method)
	assert.Equal(t, 2, record.PreCheckpoints)
	assert.Equal(t, 1, record.PostCheckpoints)
	assert.Equal(t, ompprobe.AttemptCoverageUnknown, record.AttemptCoverage)
	assert.Empty(t, record.AbortReason)
	assert.False(t, record.MaintenanceUsagePresent,
		"a completed compaction whose cost was never reported stays unknown, not free")
}

// omp/18.1.x evaluates the reduction after it has fired its pre-compaction
// hooks, so a decline can arrive with both pre-checkpoints acknowledged. That
// is a measurement, not a protocol violation — but its no-op proof still has
// to hold, and neither an unchanged transcript nor an idle session may be
// assumed.
func TestPipelineOMPActiveManualCompactProvesDeclineAfterRepeatedPre(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		variant string
		abort   string
		outcome string
		refusal string
	}{
		{
			name:    "a decline after two acknowledged pre-checkpoints is a measurement",
			variant: pipelineOMPActiveCheckpointRefuse,
			abort:   "",
			outcome: ompprobe.OutcomeRefused,
			refusal: "too_small",
		},
		{
			name:    "a decline whose transcript moved fails its no-op proof",
			variant: pipelineOMPActiveCheckpointRefuseMutate,
			abort:   "noop_proof_invalid",
			outcome: ompprobe.OutcomeFailed,
			refusal: "too_small",
		},
		{
			name:    "a decline on a session that is still working fails its no-op proof",
			variant: pipelineOMPActiveCheckpointRefuseBusy,
			abort:   "noop_proof_invalid",
			outcome: ompprobe.OutcomeFailed,
			refusal: "too_small",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			exchange := runPipelineOMPActiveCheckpointExchange(t, testCase.variant)

			assert.False(t, exchange.performed, "a declined compaction never reports a completion")
			if testCase.abort == "" {
				require.NoError(t, exchange.err)
			} else {
				require.Error(t, exchange.err)
			}
			assert.Equal(t, testCase.abort, exchange.record.AbortReason)
			assert.Equal(t, testCase.outcome, exchange.record.Outcome)
			assert.Equal(t, testCase.refusal, exchange.record.Refusal)
			assert.Equal(t, ompprobe.MethodNone, exchange.record.Method,
				"a declined compaction never named a native method")
			assert.Equal(t, 2, exchange.record.PreCheckpoints)
			assert.Zero(t, exchange.record.PostCheckpoints)
			assert.Equal(t, workflowContextCheckpointFirstACKs("pre-1", "pre-2"), exchange.acks)
			assert.Zero(t, exchange.readmissions,
				"a compaction that did not happen re-admits nothing")
			assert.Zero(t, exchange.loopback.compactions)
			assert.Len(t, exchange.loopback.transcript, 1, "a declined compaction leaves the history alone")
		})
	}
}
