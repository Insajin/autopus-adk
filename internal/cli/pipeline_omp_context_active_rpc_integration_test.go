package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipelineOMPActiveRPC_ReusesOneSessionAcrossManualCompaction(t *testing.T) {
	session, config, logPath := pipelineOMPActiveRPCSessionFixture(t, false)
	t.Cleanup(func() { require.NoError(t, session.Close()) })

	first, firstReceipt, err := session.Execute(context.Background(), "safe plan phase")
	require.NoError(t, err)
	second, secondReceipt, err := session.Execute(context.Background(), "safe implementation phase")
	require.NoError(t, err)

	assert.Equal(t, "safe assistant output 1", first)
	assert.Equal(t, "safe assistant output 2", second)
	assert.Equal(t, firstReceipt.SessionID, secondReceipt.SessionID)
	assert.Zero(t, firstReceipt.CompactionCycles)
	assert.Equal(t, 1, secondReceipt.CompactionCycles)
	assert.Zero(t, firstReceipt.PreCompactionACKs)
	assert.Zero(t, firstReceipt.PostCompactionACKs)
	assert.Zero(t, firstReceipt.CanonicalReadmissions)
	assert.Zero(t, firstReceipt.EphemeralReadmissions)
	assert.Equal(t, firstReceipt.SessionBindingHash, secondReceipt.SessionBindingHash)
	assert.NotEmpty(t, firstReceipt.BridgeBindingHash)
	assert.True(t, secondReceipt.SameProcess)
	assert.True(t, secondReceipt.SameSession)
	assert.Equal(t, int64(40), secondReceipt.InputTokens)
	assert.Equal(t, int64(10), secondReceipt.OutputTokens)
	assert.Zero(t, secondReceipt.MaintenanceInputTokens)
	assert.Zero(t, secondReceipt.MaintenanceOutputTokens)
	assert.Equal(t, int64(50), secondReceipt.TotalTokens)
	records := readPipelineOMPRPCRecords(t, logPath)
	starts, commands := pipelineOMPRPCRecordsByKind(records)
	require.Len(t, starts, 1)
	assert.Equal(t, 2, countPipelineOMPRPCCommand(commands, "prompt"))
	assert.Equal(t, 1, countPipelineOMPRPCCommand(commands, "compact"))
	assert.Equal(t, 4, countPipelineOMPRPCCommand(commands, "get_messages_page"))
	assertPipelineOMPRPCBooleanCommand(t, commands, "set_auto_compaction", false)
	require.NoError(t, session.Close())
	assertPipelineOMPRuntimeEmpty(t, config.RuntimeBase)
}

func TestPipelineOMPActiveRPC_AcceptsLegacyManualCompactionLifecycle(t *testing.T) {
	t.Setenv("AUTOPUS_TEST_OMP_ACTIVE_LEGACY_COMPACTION", "1")
	session, _, _ := pipelineOMPActiveRPCSessionFixture(t, false)
	t.Cleanup(func() { require.NoError(t, session.Close()) })

	_, firstReceipt, err := session.Execute(context.Background(), "safe plan phase")
	require.NoError(t, err)
	_, secondReceipt, err := session.Execute(context.Background(), "safe implementation phase")

	require.NoError(t, err)
	assert.Zero(t, firstReceipt.CompactionCycles)
	assert.Equal(t, 1, secondReceipt.CompactionCycles)
}

func TestPipelineOMPActiveRPC_UnsafeFirstOutputClosesBeforeSecondProviderCall(t *testing.T) {
	session, _, logPath := pipelineOMPActiveRPCSessionFixture(t, true)

	_, _, err := session.Execute(context.Background(), "safe plan phase")
	require.ErrorContains(t, err, "unsafe assistant output")
	_, _, err = session.Execute(context.Background(), "safe implementation phase")
	require.Error(t, err)

	_, commands := pipelineOMPRPCRecordsByKind(readPipelineOMPRPCRecords(t, logPath))
	assert.Equal(t, 1, countPipelineOMPRPCCommand(commands, "prompt"))
	assert.Zero(t, countPipelineOMPRPCCommand(commands, "compact"))
}

func TestPipelineOMPActiveRPC_AcceptsProvenEmptyNoopCompaction(t *testing.T) {
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
		{ID: "pipeline-active-compact-2", Type: "response", Command: "compact", Error: pipelineOMPActiveCompactionNoopMessages[0]},
		{ID: "pipeline-3", Type: "response", Command: "get_messages_page", Success: true, Data: page},
		{ID: "pipeline-4", Type: "response", Command: "get_state", Success: true, Data: idle},
	})
	prepareCalls := 0

	compacted, err := protocol.manualCompact(
		context.Background(), WorkflowContextBridgeBinding{}, "active-session",
		func() (string, error) {
			prepareCalls++
			return "unused", nil
		},
	)

	require.NoError(t, err)
	assert.False(t, compacted)
	assert.Zero(t, prepareCalls)
}
