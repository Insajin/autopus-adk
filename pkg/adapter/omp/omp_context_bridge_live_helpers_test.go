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

func isOMPContextBridgeNotify(frame []byte) bool {
	var value struct {
		Type    string `json:"type"`
		Method  string `json:"method"`
		Message string `json:"message"`
	}
	if json.Unmarshal(frame, &value) != nil || value.Type != "extension_ui_request" || value.Method != "notify" {
		return false
	}
	return strings.Contains(value.Message, `"schema_version":"autopus.omp-context-bridge.v1"`)
}

func assertOMPContextBridgeRPCEnvelopes(t *testing.T, frames [][]byte, hashes map[string]string) {
	t.Helper()
	seen := make(map[string]int)
	for _, frame := range frames {
		if !isOMPContextBridgeNotify(frame) {
			continue
		}
		var rpc struct {
			NotifyType string `json:"notifyType"`
			Message    string `json:"message"`
		}
		require.NoError(t, json.Unmarshal(frame, &rpc))
		assert.Equal(t, "info", rpc.NotifyType)
		var envelope map[string]string
		require.NoError(t, json.Unmarshal([]byte(rpc.Message), &envelope))
		require.Len(t, envelope, 5, "the body-free envelope has exact keys only")
		assert.Equal(t, "autopus.omp-context-bridge.v1", envelope["schema_version"])
		for key, value := range hashes {
			assert.Equal(t, value, envelope[key])
		}
		seen[envelope["event"]]++
	}
	assert.Equal(t, map[string]int{"pre_compaction": 1, "post_compaction": 1}, seen)
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
