package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipelineOMPActiveManagedPrompt_AcceptsNullResponseLifecycleAndSafeWidget(t *testing.T) {
	t.Parallel()
	terminal := true
	frames := []pipelineOMPRPCFrame{
		{ID: "pipeline-active-prompt-1", Type: "response", Command: "prompt", Success: true, Data: json.RawMessage(`null`)},
		{ID: "pipeline-active-prompt-1", Type: "prompt_result", AgentInvoked: boolPointer(true)},
		{Type: "agent_start"},
		{Type: "turn_start"},
		{Type: "turn_end"},
		{ID: "widget-1", Type: "extension_ui_request", Method: "setWidget"},
		{Type: "agent_end", IsTerminal: &terminal},
	}
	protocol, sent := pipelineOMPProtocolFixture(frames)

	err := protocol.callManagedPrompt(context.Background(), "safe prompt")

	require.NoError(t, err)
	assert.NotContains(t, sent.String(), "extension_ui_response")
}

func TestPipelineOMPActiveManagedPrompt_AcceptsLifecycleStartBeforeResponse(t *testing.T) {
	t.Parallel()
	terminal := true
	frames := []pipelineOMPRPCFrame{
		{Type: "agent_start"},
		{Type: "turn_start"},
		{ID: "widget-1", Type: "extension_ui_request", Method: "setWidget"},
		{ID: "pipeline-active-prompt-1", Type: "prompt_result", AgentInvoked: boolPointer(true)},
		{ID: "pipeline-active-prompt-1", Type: "response", Command: "prompt", Success: true, Data: json.RawMessage(`null`)},
		{Type: "turn_end"},
		{Type: "agent_end", IsTerminal: &terminal},
	}
	protocol, sent := pipelineOMPProtocolFixture(frames)

	err := protocol.callManagedPrompt(context.Background(), "safe prompt")

	require.NoError(t, err)
	assert.NotContains(t, sent.String(), "extension_ui_response")
}

func TestPipelineOMPActiveManagedPrompt_AcceptsRetryCyclesBeforeTerminalEnd(t *testing.T) {
	t.Parallel()
	terminal, nonTerminal := true, false
	success := pipelineOMPRPCFrame{
		ID: "pipeline-active-prompt-1", Type: "response", Command: "prompt", Success: true, Data: json.RawMessage(`null`),
	}
	accepted := map[string][]pipelineOMPRPCFrame{
		"held agent end": {
			success,
			{Type: "agent_start"}, {Type: "turn_start"}, {Type: "turn_end"},
			{Type: "agent_start"}, {Type: "turn_start"}, {Type: "turn_end"},
			{Type: "agent_end", IsTerminal: &terminal},
		},
		"explicit non-terminal end": {
			success,
			{Type: "agent_start"}, {Type: "turn_start"}, {Type: "turn_end"},
			{Type: "agent_end", IsTerminal: &nonTerminal},
			{Type: "agent_start"}, {Type: "turn_start"}, {Type: "turn_end"},
			{Type: "agent_end", IsTerminal: &terminal},
		},
		"response after first turn": {
			{Type: "agent_start"}, {Type: "turn_start"}, {Type: "turn_end"}, success,
			{Type: "agent_start"}, {Type: "turn_start"}, {Type: "turn_end"},
			{Type: "agent_end", IsTerminal: &terminal},
		},
	}
	for name, frames := range accepted {
		t.Run(name, func(t *testing.T) {
			protocol, _ := pipelineOMPProtocolFixture(frames)

			require.NoError(t, protocol.callManagedPrompt(context.Background(), "safe prompt"))
		})
	}
}

func TestPipelineOMPActiveManagedPrompt_RejectsInteractiveOrUncorrelatedActivity(t *testing.T) {
	t.Parallel()
	success := pipelineOMPRPCFrame{
		ID: "pipeline-active-prompt-1", Type: "response", Command: "prompt", Success: true, Data: json.RawMessage(`null`),
	}
	tests := []struct {
		name, want string
		frames     []pipelineOMPRPCFrame
	}{
		{name: "interactive confirm", want: "maintenance crossed", frames: []pipelineOMPRPCFrame{
			success, {Type: "agent_start"}, {Type: "turn_start"}, {Type: "turn_end"},
			{ID: "confirm-1", Type: "extension_ui_request", Method: "confirm"},
		}},
		{name: "fire and forget notify remains closed", want: "maintenance crossed", frames: []pipelineOMPRPCFrame{
			success, {Type: "agent_start"}, {Type: "turn_start"}, {Type: "turn_end"},
			{ID: "notify-1", Type: "extension_ui_request", Method: "notify"},
		}},
		{name: "uncorrelated result", want: "did not invoke", frames: []pipelineOMPRPCFrame{
			success, {ID: "other", Type: "prompt_result", AgentInvoked: boolPointer(true)},
		}},
		{name: "local only result", want: "did not invoke", frames: []pipelineOMPRPCFrame{
			success, {ID: "pipeline-active-prompt-1", Type: "prompt_result", AgentInvoked: boolPointer(false)},
		}},
		{name: "response after terminal end", want: "rejected", frames: []pipelineOMPRPCFrame{
			{Type: "agent_start"}, {Type: "turn_start"}, {Type: "turn_end"},
			{Type: "agent_end", IsTerminal: boolPointer(true)}, success,
		}},
		{name: "start inside an open turn", want: "primary start is out of order", frames: []pipelineOMPRPCFrame{
			success, {Type: "agent_start"}, {Type: "turn_start"}, {Type: "agent_start"},
		}},
		{name: "terminal end without a completed turn", want: "terminal event is invalid", frames: []pipelineOMPRPCFrame{
			success, {Type: "agent_start"}, {Type: "agent_end", IsTerminal: boolPointer(true)},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			protocol, _ := pipelineOMPProtocolFixture(test.frames)
			err := protocol.callManagedPrompt(context.Background(), "safe prompt")
			require.ErrorContains(t, err, test.want)
		})
	}
}
