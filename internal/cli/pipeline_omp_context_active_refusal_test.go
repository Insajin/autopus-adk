package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OMP declines a compaction in more than one shape, and treating an unknown
// refusal as a protocol violation cost the first v0.50.114 attempt its cohort:
// omp/18.1.5 answered "snapcompact would not reduce context locally." at call 6
// of 42 and the harness rejected it as an invalid response.
//
// Measured 2026-09-03 with `strings` over the staged binaries: that message is
// absent from 17.2.7 and present six times in both 18.1.2 and 18.1.5.
func TestPipelineOMPActiveCompactionRefused_CoversEveryMeasuredShape(t *testing.T) {
	t.Parallel()

	for _, refusal := range []string{
		"Nothing to compact (session too small)",
		"Nothing to compact (no messages yet)",
		"snapcompact would not reduce context locally.",
	} {
		assert.True(t, pipelineOMPActiveCompactionRefused(refusal),
			"%q is a refusal OMP actually emits, not a protocol violation", refusal)
	}
	for _, other := range []string{
		"", "compact failed", "Nothing to compact", "snapcompact would not reduce context",
		"internal error",
	} {
		assert.False(t, pipelineOMPActiveCompactionRefused(other),
			"%q is not a known refusal and must not be treated as a no-op", other)
	}
}

// 18.1.x evaluates whether compaction pays off *after* firing the
// pre-compaction hook, so a refusal can land with the pre-ACK already
// acknowledged. The no-op proof does not depend on that ordering — it re-reads
// the transcript and the idle state and requires both unchanged — so the
// pre-ACK is tolerated while the post-ACK must still be absent.
func TestManualCompact_AcceptsRefusalAfterThePreCompactionACK(t *testing.T) {
	t.Parallel()

	page, err := json.Marshal(pipelineOMPActiveMessagesPage{
		Messages: nil, TotalMessages: 0, NextCursor: nil,
	})
	require.NoError(t, err)
	idle, err := json.Marshal(map[string]any{
		"sessionId": "active-session", "isStreaming": false, "isCompacting": false,
		"messageCount": 0, "queuedMessageCount": 0, "autoCompactionEnabled": false,
	})
	require.NoError(t, err)

	protocol, _ := pipelineOMPProtocolFixture([]pipelineOMPRPCFrame{
		{ID: "pipeline-1", Type: "response", Command: "get_messages_page", Success: true, Data: page},
		{
			ID: "pipeline-active-compact-2", Type: "response", Command: "compact",
			Error: "snapcompact would not reduce context locally.",
		},
		{ID: "pipeline-3", Type: "response", Command: "get_messages_page", Success: true, Data: page},
		{ID: "pipeline-4", Type: "response", Command: "get_state", Success: true, Data: idle},
	})

	compacted, err := protocol.manualCompact(
		context.Background(), WorkflowContextBridgeBinding{}, "active-session",
		func() (string, error) { return "unused", nil },
	)

	require.NoError(t, err, "a measured refusal must not fail the managed lane")
	assert.False(t, compacted, "a refused compaction reduced nothing, so it did not compact")
}
