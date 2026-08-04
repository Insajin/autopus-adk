package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

type workflowContextManagedRPCSessionStats struct {
	SessionID string `json:"sessionId"`
	Tokens    struct {
		Input  int64 `json:"input"`
		Output int64 `json:"output"`
		Total  int64 `json:"total"`
	} `json:"tokens"`
}

// lastAssistantText reads provider output only after the caller has verified a
// complete provider lifecycle and an idle, same-session state. The body is
// returned process-privately and never enters a receipt or log.
func (protocol *workflowContextManagedRPCProtocol) lastAssistantText(
	ctx context.Context,
	expectedSession string,
) (string, error) {
	state, err := protocol.state(ctx, "managed-output-state")
	if err != nil || !safeWorkflowContextManagedRPCState(state) || state.SessionID != expectedSession {
		return "", errors.New("managed OMP output state is not session-bound")
	}
	const id = "managed-last-assistant-text"
	if err := protocol.send(map[string]any{"id": id, "type": "get_last_assistant_text"}); err != nil {
		return "", err
	}
	frame, err := protocol.awaitResponse(ctx, id)
	if err != nil || frame.Command != "get_last_assistant_text" {
		return "", errors.New("managed OMP assistant output is unavailable")
	}
	var result struct {
		Text string `json:"text"`
	}
	if len(frame.Data) == 0 || json.Unmarshal(frame.Data, &result) != nil || strings.TrimSpace(result.Text) == "" {
		return "", errors.New("managed OMP assistant output is unavailable")
	}
	return result.Text, nil
}

// sessionStats reads cumulative usage only from an idle, same-session OMP RPC
// stream. The result is retained in memory and never enters a receipt.
func (protocol *workflowContextManagedRPCProtocol) sessionStats(
	ctx context.Context,
	expectedSession string,
	id string,
) (workflowContextManagedRPCSessionStats, error) {
	state, err := protocol.state(ctx, id+"-state")
	if err != nil || !safeWorkflowContextManagedRPCState(state) || state.SessionID != expectedSession {
		return workflowContextManagedRPCSessionStats{}, errors.New("managed OMP stats state is not session-bound")
	}
	if err := protocol.send(map[string]any{"id": id, "type": "get_session_stats"}); err != nil {
		return workflowContextManagedRPCSessionStats{}, err
	}
	frame, err := protocol.awaitResponse(ctx, id)
	if err != nil || frame.Command != "get_session_stats" {
		return workflowContextManagedRPCSessionStats{}, errors.New("managed OMP session stats are unavailable")
	}
	var stats workflowContextManagedRPCSessionStats
	if len(frame.Data) == 0 || json.Unmarshal(frame.Data, &stats) != nil || stats.SessionID != expectedSession ||
		stats.Tokens.Input < 0 || stats.Tokens.Output < 0 || stats.Tokens.Total < 0 {
		return workflowContextManagedRPCSessionStats{}, errors.New("managed OMP session stats are unavailable")
	}
	return stats, nil
}

func workflowContextProviderUsageDelta(
	before workflowContextManagedRPCSessionStats,
	after workflowContextManagedRPCSessionStats,
) (WorkflowContextProviderUsage, error) {
	if before.SessionID == "" || before.SessionID != after.SessionID ||
		after.Tokens.Input < before.Tokens.Input || after.Tokens.Output < before.Tokens.Output {
		return WorkflowContextProviderUsage{}, errors.New("managed OMP provider usage is not monotonic")
	}
	primaryInput := after.Tokens.Input - before.Tokens.Input
	primaryOutput := after.Tokens.Output - before.Tokens.Output
	if primaryInput <= 0 || primaryOutput <= 0 {
		return WorkflowContextProviderUsage{}, errors.New("managed OMP primary provider usage is unavailable")
	}
	return WorkflowContextProviderUsage{
		PrimaryInputTokens: primaryInput, PrimaryOutputTokens: primaryOutput,
		MaintenanceInputTokens: before.Tokens.Input, MaintenanceOutputTokens: before.Tokens.Output,
		TotalTokens: after.Tokens.Input + after.Tokens.Output,
	}, nil
}
