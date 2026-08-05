package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

func (protocol *pipelineOMPRPCProtocol) executeManaged(
	ctx context.Context,
	model string,
	binding WorkflowContextBridgeBinding,
	expectedSession string,
	compactBefore bool,
	preparePrompt func() (string, error),
) (string, pipelineOMPActiveCallReceipt, error) {
	provider, modelID, ok := strings.Cut(model, "/")
	if !ok || !safePipelineOMPToken(provider) || !safePipelineOMPToken(modelID) || preparePrompt == nil {
		return "", pipelineOMPActiveCallReceipt{}, errors.New("managed active OMP phase authority is invalid")
	}
	if _, err := protocol.call(ctx, pipelineOMPRPCCommand{Type: "set_model", Provider: provider, ModelID: modelID}, false); err != nil {
		return "", pipelineOMPActiveCallReceipt{}, err
	}
	beforeState, err := protocol.readIdleState(ctx, "managed-pre-prompt")
	if err != nil || beforeState.SessionID != expectedSession {
		return "", pipelineOMPActiveCallReceipt{}, errors.New("managed active OMP pre-prompt session changed")
	}
	var preparedPrompt string
	promptReady := false
	rehydrationCalls := 0
	rehydrate := func() (string, error) {
		if promptReady {
			return preparedPrompt, nil
		}
		body, prepareErr := preparePrompt()
		if prepareErr != nil {
			return "", prepareErr
		}
		rehydrationCalls++
		preparedPrompt, promptReady = body, true
		return preparedPrompt, nil
	}
	maintenanceInput, maintenanceOutput := int64(0), int64(0)
	maintenanceTotal := int64(0)
	if compactBefore {
		beforeMaintenance, statsErr := protocol.sessionStats(ctx, expectedSession)
		if statsErr != nil {
			return "", pipelineOMPActiveCallReceipt{}, statsErr
		}
		if err := protocol.manualCompact(ctx, binding, expectedSession, rehydrate); err != nil {
			return "", pipelineOMPActiveCallReceipt{}, err
		}
		if !promptReady || rehydrationCalls != 1 {
			return "", pipelineOMPActiveCallReceipt{}, errors.New("managed active OMP post-compaction re-admission is incomplete")
		}
		afterMaintenance, statsErr := protocol.sessionStats(ctx, expectedSession)
		if statsErr != nil || afterMaintenance.Input < beforeMaintenance.Input ||
			afterMaintenance.Output < beforeMaintenance.Output || afterMaintenance.Total < beforeMaintenance.Total {
			return "", pipelineOMPActiveCallReceipt{}, errors.New("managed active OMP maintenance usage is not monotonic")
		}
		maintenanceInput = afterMaintenance.Input - beforeMaintenance.Input
		maintenanceOutput = afterMaintenance.Output - beforeMaintenance.Output
		maintenanceTotal = afterMaintenance.Total - beforeMaintenance.Total
	}
	prompt, err := rehydrate()
	if err != nil {
		return "", pipelineOMPActiveCallReceipt{}, err
	}
	beforeStats, err := protocol.sessionStats(ctx, expectedSession)
	if err != nil {
		return "", pipelineOMPActiveCallReceipt{}, err
	}
	err = protocol.callManagedPrompt(ctx, prompt)
	if err != nil {
		return "", pipelineOMPActiveCallReceipt{}, err
	}
	afterState, err := protocol.readIdleState(ctx, "managed-post-prompt")
	if err != nil || afterState.SessionID != expectedSession || *afterState.MessageCount <= *beforeState.MessageCount {
		return "", pipelineOMPActiveCallReceipt{}, errors.New("managed active OMP primary did not settle in the same session")
	}
	afterStats, err := protocol.sessionStats(ctx, expectedSession)
	if err != nil || afterStats.Input < beforeStats.Input || afterStats.Output < beforeStats.Output ||
		afterStats.Total < beforeStats.Total {
		return "", pipelineOMPActiveCallReceipt{}, errors.New("managed active OMP usage is not monotonic")
	}
	data, err := protocol.call(ctx, pipelineOMPRPCCommand{Type: "get_last_assistant_text"}, false)
	if err != nil {
		return "", pipelineOMPActiveCallReceipt{}, err
	}
	var output struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(data, &output) != nil || strings.TrimSpace(output.Text) == "" {
		return "", pipelineOMPActiveCallReceipt{}, errors.New("managed active OMP returned empty assistant output")
	}
	if err := validatePipelineOMPActiveText(output.Text); err != nil {
		return "", pipelineOMPActiveCallReceipt{}, errors.New("managed active OMP returned unsafe assistant output")
	}
	if _, _, err := protocol.validatePipelineOMPActiveTranscript(ctx, false); err != nil {
		return "", pipelineOMPActiveCallReceipt{}, err
	}
	inputDelta, outputDelta := afterStats.Input-beforeStats.Input, afterStats.Output-beforeStats.Output
	totalDelta := afterStats.Total - beforeStats.Total
	if inputDelta <= 0 || outputDelta <= 0 || totalDelta < inputDelta+outputDelta {
		return "", pipelineOMPActiveCallReceipt{}, errors.New("managed active OMP usage delta is empty")
	}
	return output.Text, pipelineOMPActiveCallReceipt{
		SessionID: expectedSession, InputTokens: inputDelta, OutputTokens: outputDelta,
		MaintenanceInputTokens: maintenanceInput, MaintenanceOutputTokens: maintenanceOutput,
		TotalTokens:           totalDelta + maintenanceTotal,
		CompactionCycles:      boolToPipelineOMPCount(compactBefore),
		PreCompactionACKs:     boolToPipelineOMPCount(compactBefore),
		PostCompactionACKs:    boolToPipelineOMPCount(compactBefore),
		CanonicalReadmissions: boolToPipelineOMPCount(compactBefore),
		EphemeralReadmissions: boolToPipelineOMPCount(compactBefore),
		SameProcess:           true, SameSession: true, TerminalIdle: true,
		BridgeBindingHash:  binding.BindingHash,
		SessionBindingHash: workflowContextRuntimeHash(expectedSession + "\x00" + binding.NonceHash),
	}, nil
}
