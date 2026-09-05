package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: protocol v2 is required for bounded rpc_chunk reconstruction.
const pipelineOMPRPCProtocolVersion = 2

type pipelineOMPRPCCommand struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	ProtocolVersion    int    `json:"protocolVersion,omitempty"`
	Enabled            *bool  `json:"enabled,omitempty"`
	Provider           string `json:"provider,omitempty"`
	ModelID            string `json:"modelId,omitempty"`
	Level              string `json:"level,omitempty"`
	Message            string `json:"message,omitempty"`
	CustomInstructions string `json:"customInstructions,omitempty"`
	Confirmed          *bool  `json:"confirmed,omitempty"`
	Cursor             string `json:"cursor,omitempty"`
	Limit              int    `json:"limit,omitempty"`
}

type pipelineOMPRPCFrame struct {
	ID           string          `json:"id,omitempty"`
	Type         string          `json:"type"`
	Command      string          `json:"command,omitempty"`
	Success      bool            `json:"success,omitempty"`
	Error        string          `json:"error,omitempty"`
	AgentInvoked *bool           `json:"agentInvoked,omitempty"`
	Data         json.RawMessage `json:"data,omitempty"`
	Method       string          `json:"method,omitempty"`
	Title        string          `json:"title,omitempty"`
	Message      json.RawMessage `json:"message,omitempty"`
	Reason       string          `json:"reason,omitempty"`
	Action       string          `json:"action,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	Result       json.RawMessage `json:"result,omitempty"`
	Aborted      bool            `json:"aborted,omitempty"`
	Skipped      bool            `json:"skipped,omitempty"`
	IsTerminal   *bool           `json:"isTerminal,omitempty"`
	Messages     json.RawMessage `json:"messages,omitempty"`
}

type pipelineOMPRPCProtocol struct {
	process              *pipelineOMPProcess
	nextID               int
	safeCompactionImages map[string]struct{}
	// declaredContextWindow is caller-asserted; zero disables executeManaged exhaustion diagnosis.
	declaredContextWindow int
	// lastTurn is the assistant identity reported by the most recent agent_end.
	lastTurn pipelineOMPTurnIdentity
}

type pipelineOMPModelState struct {
	Provider string   `json:"provider"`
	ID       string   `json:"id"`
	Input    []string `json:"input"`
}

type pipelineOMPState struct {
	SessionID             string                 `json:"sessionId"`
	IsStreaming           *bool                  `json:"isStreaming"`
	IsCompacting          *bool                  `json:"isCompacting"`
	MessageCount          *int                   `json:"messageCount"`
	QueuedMessageCount    *int                   `json:"queuedMessageCount"`
	AutoCompactionEnabled *bool                  `json:"autoCompactionEnabled"`
	Model                 *pipelineOMPModelState `json:"model"`
}

func (state pipelineOMPState) supportsNativeImageCompaction(selector string) bool {
	provider, modelID, ok := strings.Cut(selector, "/")
	if !ok || state.Model == nil || state.Model.Provider != provider || state.Model.ID != modelID {
		return false
	}
	text, image := false, false
	for _, input := range state.Model.Input {
		text = text || input == "text"
		image = image || input == "image"
	}
	return text && image
}

func newPipelineOMPRPCProtocol(process *pipelineOMPProcess) *pipelineOMPRPCProtocol {
	return &pipelineOMPRPCProtocol{process: process, safeCompactionImages: make(map[string]struct{})}
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: negotiation disables OMP retry and automatic compaction before phase prompts.
// @AX:REASON [AUTO]: The pipeline engine, rather than OMP session defaults, owns retry and compaction authority.
func (protocol *pipelineOMPRPCProtocol) initialize(ctx context.Context) error {
	data, err := protocol.call(ctx, pipelineOMPRPCCommand{
		Type: "negotiate_protocol", ProtocolVersion: pipelineOMPRPCProtocolVersion,
	}, false)
	if err != nil {
		return err
	}
	var negotiated struct {
		ProtocolVersion int `json:"protocolVersion"`
	}
	if json.Unmarshal(data, &negotiated) != nil || negotiated.ProtocolVersion != pipelineOMPRPCProtocolVersion {
		return errors.New("OMP pipeline RPC protocol v2 was not negotiated")
	}
	disabled := false
	for _, commandType := range []string{"set_auto_retry", "set_auto_compaction"} {
		if _, err := protocol.call(ctx, pipelineOMPRPCCommand{Type: commandType, Enabled: &disabled}, false); err != nil {
			return err
		}
	}
	return nil
}

func (protocol *pipelineOMPRPCProtocol) execute(
	ctx context.Context,
	model string,
	prompt string,
) (string, error) {
	if model != "" {
		provider, modelID, ok := strings.Cut(model, "/")
		if !ok || !safePipelineOMPToken(provider) || !safePipelineOMPToken(modelID) {
			return "", fmt.Errorf("invalid OMP pipeline model selector")
		}
		if _, err := protocol.call(ctx, pipelineOMPRPCCommand{
			Type: "set_model", Provider: provider, ModelID: modelID,
		}, false); err != nil {
			return "", err
		}
	}
	before, err := protocol.readIdleState(ctx, "pre-prompt")
	if err != nil {
		return "", err
	}
	if _, err := protocol.call(ctx, pipelineOMPRPCCommand{Type: "prompt", Message: prompt}, true); err != nil {
		return "", err
	}
	after, err := protocol.readIdleState(ctx, "post-prompt")
	if err != nil {
		return "", err
	}
	if after.SessionID != before.SessionID || *after.MessageCount <= *before.MessageCount {
		return "", errors.New("OMP pipeline RPC post-prompt state is not bound to a completed turn")
	}
	data, err := protocol.call(ctx, pipelineOMPRPCCommand{Type: "get_last_assistant_text"}, false)
	if err != nil {
		return "", err
	}
	var output struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(data, &output) != nil || strings.TrimSpace(output.Text) == "" {
		return "", errors.New("OMP pipeline RPC returned empty assistant output")
	}
	return output.Text, nil
}

func (protocol *pipelineOMPRPCProtocol) setThinkingLevel(ctx context.Context, level string) error {
	_, err := protocol.call(ctx, pipelineOMPRPCCommand{Type: "set_thinking_level", Level: level}, false)
	return err
}

func (protocol *pipelineOMPRPCProtocol) readIdleState(
	ctx context.Context,
	stage string,
) (pipelineOMPState, error) {
	data, err := protocol.call(ctx, pipelineOMPRPCCommand{Type: "get_state"}, false)
	if err != nil {
		return pipelineOMPState{}, err
	}
	var state pipelineOMPState
	if json.Unmarshal(data, &state) != nil || strings.TrimSpace(state.SessionID) == "" ||
		state.IsStreaming == nil || state.IsCompacting == nil || state.MessageCount == nil ||
		state.QueuedMessageCount == nil || *state.IsStreaming || *state.IsCompacting ||
		*state.MessageCount < 0 || *state.QueuedMessageCount != 0 {
		return pipelineOMPState{}, fmt.Errorf("OMP pipeline RPC did not reach an idle %s state", stage)
	}
	return state, nil
}

// @AX:WARN [AUTO]: RPC response correlation and prompt lifecycle admission have cyclomatic complexity 30.
// @AX:REASON [AUTO]: Responses, lifecycle events, and extension failures may arrive in different orders but must bind to one command.
func (protocol *pipelineOMPRPCProtocol) call(
	ctx context.Context,
	command pipelineOMPRPCCommand,
	waitLifecycle bool,
) (json.RawMessage, error) {
	protocol.nextID++
	command.ID = fmt.Sprintf("pipeline-%d", protocol.nextID)
	if err := protocol.process.send(command); err != nil {
		return nil, err
	}
	var data json.RawMessage
	responded, started, ended := false, false, false
	for {
		frame, err := protocol.process.next(ctx)
		if err != nil {
			return nil, err
		}
		if frame.Type == "extension_error" {
			return nil, fmt.Errorf("OMP pipeline extension error: %s", frame.Error)
		}
		if frame.Type == "response" && frame.ID == command.ID {
			if !frame.Success {
				return nil, fmt.Errorf("OMP pipeline RPC %s failed: %s", command.Type, frame.Error)
			}
			if frame.Command != command.Type {
				return nil, fmt.Errorf("OMP pipeline RPC response command mismatch")
			}
			if waitLifecycle && !bareOMPPromptAck(frame.Data) {
				// omp 17.x acknowledges a prompt with {agentInvoked:true}; omp
				// 18.1.x sends a bare success and proves the run through the
				// agent_start/agent_end lifecycle frames awaited below.
				var result struct {
					AgentInvoked *bool `json:"agentInvoked"`
				}
				if json.Unmarshal(frame.Data, &result) != nil {
					return nil, errors.New("OMP pipeline RPC prompt result is malformed")
				}
				if result.AgentInvoked == nil || !*result.AgentInvoked {
					return nil, errors.New("OMP pipeline RPC prompt completed without invoking the agent")
				}
			}
			data, responded = append(json.RawMessage(nil), frame.Data...), true
		}
		if waitLifecycle {
			if frame.Type == "prompt_result" {
				if frame.ID != command.ID || frame.AgentInvoked == nil || !*frame.AgentInvoked {
					return nil, errors.New("OMP pipeline RPC prompt did not enter the agent lifecycle")
				}
			}
			switch frame.Type {
			case "agent_start":
				if ended {
					return nil, errors.New("OMP pipeline RPC agent lifecycle is ambiguous")
				}
				started = true
			case "agent_end":
				if !started || ended {
					return nil, errors.New("OMP pipeline RPC agent lifecycle is out of order")
				}
				// A retry re-enters the agent loop, so only a terminal end settles the prompt.
				ended = frame.IsTerminal == nil || *frame.IsTerminal
				if ended {
					identity, err := settlePipelineOMPTurn(frame.Messages)
					if err != nil {
						return nil, err
					}
					protocol.lastTurn = identity
				}
			}
			if responded && started && ended {
				return data, nil
			}
		} else if responded {
			return data, nil
		}
	}
}
