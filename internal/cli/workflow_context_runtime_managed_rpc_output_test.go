package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextManagedRPCOutput_ReadsOnlyAfterSameSessionIdleStateWithoutACKLeak(t *testing.T) {
	frames := make(chan []byte, 2)
	done := make(chan error, 1)
	stateData, err := json.Marshal(map[string]any{
		"sessionId": "session-1", "isStreaming": false, "isCompacting": false,
		"autoCompactionEnabled": false, "queuedMessageCount": 0, "messageCount": 4,
	})
	require.NoError(t, err)
	outputData, err := json.Marshal(map[string]string{"text": "private-provider-output"})
	require.NoError(t, err)
	frames <- workflowContextManagedOutputFrame(t, "managed-output-state", "get_state", stateData)
	frames <- workflowContextManagedOutputFrame(t, "managed-last-assistant-text", "get_last_assistant_text", outputData)
	close(frames)
	var commands bytes.Buffer
	protocol := newWorkflowContextManagedRPCProtocol(&commands, frames, done)

	output, err := protocol.lastAssistantText(context.Background(), "session-1")

	require.NoError(t, err)
	assert.Equal(t, "private-provider-output", output)
	assert.Contains(t, commands.String(), `"type":"get_last_assistant_text"`)
	ack := validWorkflowContextDispatchAck(WorkflowContextBridgeBinding{
		SchemaVersion: workflowContextBridgeSchemaVersion,
		BindingHash:   workflowContextRuntimeHash("binding"), OptionsHash: workflowContextRuntimeHash("options"),
		SessionHash: workflowContextRuntimeHash("session"), NonceHash: workflowContextRuntimeHash("nonce"),
	})
	ack.providerOutput = output
	encoded, err := json.Marshal(ack)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), output)
}

func TestWorkflowContextManagedRPCOutput_RejectsWrongSessionAndEmptyBody(t *testing.T) {
	for _, test := range []struct {
		name, actualSession, output string
	}{
		{name: "wrong session", actualSession: "session-other", output: "text"},
		{name: "empty output", actualSession: "session-1", output: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			frames := make(chan []byte, 2)
			done := make(chan error, 1)
			state, _ := json.Marshal(map[string]any{
				"sessionId": test.actualSession, "isStreaming": false, "isCompacting": false,
				"autoCompactionEnabled": false, "queuedMessageCount": 0, "messageCount": 4,
			})
			frames <- workflowContextManagedOutputFrame(t, "managed-output-state", "get_state", state)
			if test.actualSession == "session-1" {
				body, _ := json.Marshal(map[string]string{"text": test.output})
				frames <- workflowContextManagedOutputFrame(t, "managed-last-assistant-text", "get_last_assistant_text", body)
			}
			close(frames)
			protocol := newWorkflowContextManagedRPCProtocol(&bytes.Buffer{}, frames, done)
			_, err := protocol.lastAssistantText(context.Background(), "session-1")
			require.Error(t, err)
		})
	}
}

func workflowContextManagedOutputFrame(t *testing.T, id, command string, data []byte) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"id": id, "type": "response", "command": command, "success": true, "data": json.RawMessage(data),
	})
	require.NoError(t, err)
	return encoded
}
