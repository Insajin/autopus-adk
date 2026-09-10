package cli

// AC-013 rejection half. Every interleaving REQ-CHECKPOINT-001 names as
// forbidden must fail the transaction before the pending checkpoint is
// acknowledged, keep the received-event counts it already observed, and leave
// the run partial: records retained, no report, no evidence, no signature.
//
// Counts are authenticated received events. A replayed or out-of-order
// checkpoint is authenticated, so it is counted and refused; a forged one is
// neither counted nor answered.

import (
	"fmt"
	"os"
	"testing"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every rejection happens in the first compact transaction of the first
// segment, so each acknowledgement the harness may legitimately have sent
// before it refused carries sequence 1.
func workflowContextCheckpointFirstACKs(suffixes ...string) []string {
	acks := make([]string, 0, len(suffixes))
	for _, suffix := range suffixes {
		acks = append(acks, fmt.Sprintf("active-checkpoint-1-%s", suffix))
	}
	return acks
}

func TestWorkflowContextObserveSession_ProbeRejectsCheckpointInterleavings(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		variant string
		abort   string
		pre     int
		post    int
		acks    []string
		// uiMethods is asserted only where the recorded UI activity is the
		// point of the case.
		uiMethods []string
	}{
		{
			name:    "a replayed request id is not a second checkpoint",
			variant: pipelineOMPActiveCheckpointReplay,
			abort:   "checkpoint_id_replayed",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "a third pre-checkpoint exceeds the bound",
			variant: pipelineOMPActiveCheckpointThird,
			abort:   "pre_ack_out_of_order",
			pre:     3,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1", "pre-2"),
		},
		{
			name:    "a transcript that moved before the second acknowledgement",
			variant: pipelineOMPActiveCheckpointMutate,
			abort:   "pre_proof_changed",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "a forged nonce is neither counted nor acknowledged",
			variant: pipelineOMPActiveCheckpointForged,
			abort:   "bridge_rejected",
			pre:     1,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "rehydration may not precede the pre-compaction checkpoint",
			variant: pipelineOMPActiveCheckpointPostFirst,
			abort:   "post_ack_out_of_order",
			pre:     0,
			post:    1,
			acks:    workflowContextCheckpointFirstACKs(),
		},
		{
			name:    "a duplicate post-checkpoint",
			variant: pipelineOMPActiveCheckpointPostDup,
			abort:   "post_ack_out_of_order",
			pre:     1,
			post:    2,
			acks:    workflowContextCheckpointFirstACKs("pre-1", "post"),
		},
		{
			name:    "a pre-checkpoint after the post is past its window",
			variant: pipelineOMPActiveCheckpointPreAfterPost,
			abort:   "pre_ack_out_of_order",
			pre:     2,
			post:    1,
			acks:    workflowContextCheckpointFirstACKs("pre-1", "post"),
		},
		{
			name:    "provider activity crosses the barrier during the proof query",
			variant: pipelineOMPActiveCheckpointProvider,
			abort:   "barrier_crossed",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "orphan turn start before proof is rejected",
			variant: pipelineOMPActiveCheckpointTurnBefore,
			abort:   "barrier_crossed",
			acks:    workflowContextCheckpointFirstACKs(),
		},
		{
			name:    "a checkpoint inside the proof query is counted, never acknowledged",
			variant: pipelineOMPActiveCheckpointExtra,
			abort:   "checkpoint_during_proof",
			pre:     3,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "an early compact response does not settle the proof query",
			variant: pipelineOMPActiveCheckpointEarly,
			abort:   "proof_query_unanswered",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "an unrelated response inside the barrier is never discarded",
			variant: pipelineOMPActiveCheckpointStray,
			abort:   "proof_query_unanswered",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "an extension error inside the proof query fails the transaction",
			variant: pipelineOMPActiveCheckpointExtError,
			abort:   "extension_failed",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:      "a notice inside the proof query is not the page answer",
			variant:   pipelineOMPActiveCheckpointNotice,
			abort:     "bridge_rejected",
			pre:       2,
			post:      0,
			acks:      workflowContextCheckpointFirstACKs("pre-1"),
			uiMethods: []string{"confirm", "notify"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			run := runWorkflowContextObserveCheckpoint(t, workflowContextCheckpointRequest{
				variant: testCase.variant,
			})

			require.Error(t, run.err)
			assert.NotErrorIs(t, run.err, errWorkflowContextObserveSessionProbeCompleted)
			compactions := workflowContextObserveProbeRecordsOfKind(run.records, "compaction")
			require.Len(t, compactions, 1, "the cohort stops at the transaction it could not admit")
			record := compactions[0]
			assert.Equal(t, ompprobe.OutcomeFailed, record.Outcome)
			assert.Equal(t, testCase.abort, record.AbortReason)
			assert.Equal(t, testCase.pre, record.PreCheckpoints,
				"authenticated pre-checkpoints received, including the refused one")
			assert.Equal(t, testCase.post, record.PostCheckpoints)
			assert.Equal(t, ompprobe.AttemptCoverageUnknown, record.AttemptCoverage)
			assert.Equal(t, ompprobe.MethodSnapcompact, record.Method,
				"the native start was admitted; the checkpoint exchange is what failed")
			assert.Equal(t, testCase.acks, run.acks, "the refused checkpoint was never acknowledged")
			if len(testCase.uiMethods) > 0 {
				assert.Equal(t, testCase.uiMethods, record.UIRequestMethods,
					"the frame was recorded by method name and never answered")
			}
			assertWorkflowContextCheckpointRemainsPartial(t, run)
		})
	}
}

// assertWorkflowContextCheckpointRemainsPartial holds the line the two live T0
// runs already established: an interrupted probe keeps what it measured, names
// why it stopped, and is never upgraded into a result.
func assertWorkflowContextCheckpointRemainsPartial(t *testing.T, run workflowContextCheckpointRun) {
	t.Helper()
	calls := workflowContextObserveProbeRecordsOfKind(run.records, "call")
	require.NotEmpty(t, calls, "an interrupted probe still retains its records")
	failed := calls[len(calls)-1]
	assert.NotEmpty(t, failed.AbortReason, "the call in flight names why it stopped")
	assert.True(t, ompprobe.ValidIdentifier(failed.AbortReason), "abort reasons stay body-free tokens")
	last := run.responses[len(run.responses)-1]
	assert.Equal(t, "error", last.Type)
	assert.NotEmpty(t, last.ErrorCode)
	assert.NotEqual(t, "probe_completed", last.ErrorCode)
	assertWorkflowContextCheckpointProducedNoEvidence(t, run)
	info, err := os.Stat(run.probePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
