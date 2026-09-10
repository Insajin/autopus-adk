package cli

// The full adversarial matrix on the real protocol. The cohort tests prove the
// same refusals end to end; this one proves the acknowledgement that must not
// exist was never even written, because the transport records every attempted
// write synchronously before the transaction can fail.

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipelineOMPActiveManualCompactRefusesCheckpointInterleavings(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		variant string
		abort   string
		pre     int
		post    int
		acks    []string
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
			name:    "an orphan turn start before any proof query",
			variant: pipelineOMPActiveCheckpointTurnBefore,
			abort:   "barrier_crossed",
			pre:     0,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs(),
		},
		{
			name:    "an agent start inside the proof query",
			variant: pipelineOMPActiveCheckpointProvider,
			abort:   "barrier_crossed",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "a turn start inside the proof query",
			variant: pipelineOMPActiveCheckpointTurnStart,
			abort:   "barrier_crossed",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "a turn end inside the proof query",
			variant: pipelineOMPActiveCheckpointTurnEnd,
			abort:   "barrier_crossed",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "an agent end inside the proof query",
			variant: pipelineOMPActiveCheckpointAgentEnd,
			abort:   "barrier_crossed",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "a prompt result inside the proof query",
			variant: pipelineOMPActiveCheckpointPromptResult,
			abort:   "barrier_crossed",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "a pre-checkpoint inside the proof query is counted, never acknowledged",
			variant: pipelineOMPActiveCheckpointExtra,
			abort:   "checkpoint_during_proof",
			pre:     3,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "a post-checkpoint inside the proof query is counted, never acknowledged",
			variant: pipelineOMPActiveCheckpointPostDuringProof,
			abort:   "checkpoint_during_proof",
			pre:     2,
			post:    1,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "a notice is not the page answer either",
			variant: pipelineOMPActiveCheckpointNotice,
			abort:   "bridge_rejected",
			pre:     2,
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
			name:    "the query id answering a different command",
			variant: pipelineOMPActiveCheckpointWrongCommand,
			abort:   "proof_query_unanswered",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "the right command under an id this query never used",
			variant: pipelineOMPActiveCheckpointWrongID,
			abort:   "proof_query_unanswered",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
		{
			name:    "an extension error inside the proof query",
			variant: pipelineOMPActiveCheckpointExtError,
			abort:   "extension_failed",
			pre:     2,
			post:    0,
			acks:    workflowContextCheckpointFirstACKs("pre-1"),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			exchange := runPipelineOMPActiveCheckpointExchange(t, testCase.variant)

			require.Error(t, exchange.err)
			assert.False(t, exchange.performed)
			assert.Equal(t, testCase.abort, exchange.record.AbortReason)
			assert.Equal(t, ompprobe.OutcomeFailed, exchange.record.Outcome)
			assert.Equal(t, ompprobe.MethodSnapcompact, exchange.record.Method,
				"the native start was admitted; the checkpoint exchange is what failed")
			assert.Equal(t, testCase.pre, exchange.record.PreCheckpoints,
				"authenticated pre-checkpoints received, including the refused one")
			assert.Equal(t, testCase.post, exchange.record.PostCheckpoints)
			assert.Equal(t, ompprobe.AttemptCoverageUnknown, exchange.record.AttemptCoverage)
			assert.Equal(t, testCase.acks, exchange.acks,
				"the refused checkpoint was never written to the session")
			assert.Zero(t, exchange.loopback.compactions,
				"a refused transaction leaves the history and the usage alone")
		})
	}
}
