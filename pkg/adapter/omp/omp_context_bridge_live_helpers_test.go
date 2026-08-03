package omp

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func printableOMPFrame(frame []byte) string {
	if len(frame) > 2048 {
		return fmt.Sprintf("%s...", frame[:2048])
	}
	return string(frame)
}

func ompRPCFrameIsSuccessfulResponse(frame []byte, id string) bool {
	var value struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Success *bool  `json:"success"`
	}
	return json.Unmarshal(frame, &value) == nil && value.ID == id && value.Type == "response" &&
		value.Success != nil && *value.Success
}

func ompRPCFrameHasCommand(frame []byte, want string) bool {
	if rpcFrameType(frame) != "available_commands_update" {
		return false
	}
	var value struct {
		Commands []json.RawMessage `json:"commands"`
	}
	if json.Unmarshal(frame, &value) != nil {
		return false
	}
	for _, raw := range value.Commands {
		var name string
		if json.Unmarshal(raw, &name) == nil && name == want {
			return true
		}
		var command struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(raw, &command) == nil && command.Name == want {
			return true
		}
	}
	return false
}

func summarizeOMPFrames(frames [][]byte) string {
	parts := make([]string, 0, len(frames))
	for _, frame := range frames {
		parts = append(parts, printableOMPFrame(frame))
	}
	return strings.Join(parts, " | ")
}

type ompContextBridgeConfirmRequest struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Method  string `json:"method"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

func parseOMPContextBridgeConfirm(frame []byte) (ompContextBridgeConfirmRequest, bool) {
	var value ompContextBridgeConfirmRequest
	if json.Unmarshal(frame, &value) != nil || value.Type != "extension_ui_request" ||
		value.Method != "confirm" || value.ID == "" {
		return ompContextBridgeConfirmRequest{}, false
	}
	if !strings.Contains(value.Message, `"schema_version":"autopus.omp-context-bridge.v1"`) {
		return ompContextBridgeConfirmRequest{}, false
	}
	return value, true
}

func assertOMPContextBridgeRPCEnvelopes(t *testing.T, frames [][]byte, hashes map[string]string) []string {
	t.Helper()
	seen := make(map[string]int)
	ids := make([]string, 0, 2)
	for _, frame := range frames {
		request, ok := parseOMPContextBridgeConfirm(frame)
		if !ok {
			continue
		}
		var envelope map[string]string
		require.NoError(t, json.Unmarshal([]byte(request.Message), &envelope))
		require.Len(t, envelope, 6, "the body-free envelope has exact keys only")
		assert.Equal(t, "autopus.omp-context-bridge.v1", envelope["schema_version"])
		for key, value := range hashes {
			assert.Equal(t, value, envelope[key])
		}
		event := envelope["event"]
		assert.Equal(t, "Autopus context "+event, request.Title)
		ids = append(ids, request.ID)
		seen[event]++
	}
	assert.Equal(t, map[string]int{"pre_compaction": 1, "post_compaction": 1}, seen)
	require.Len(t, ids, 2)
	assert.NotEqual(t, ids[0], ids[1], "each ACK request ID must be single-use")
	return ids
}

func assertNoOMPProviderActivity(t *testing.T, frames [][]byte) {
	t.Helper()
	for _, frame := range frames {
		frameType := rpcFrameType(frame)
		switch frameType {
		case "agent_start", "message_start", "message_update", "message_end",
			"tool_execution_start", "auto_compaction_start":
			t.Errorf("unexpected provider/agent activity %q: %s", frameType, printableOMPFrame(frame))
		case "extension_error":
			t.Errorf("installed extension runtime error: %s", printableOMPFrame(frame))
		}
	}
}
