package cli

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

type pipelineOMPActiveCallReceipt struct {
	SessionID               string
	Sequence                int
	InputTokens             int64
	OutputTokens            int64
	MaintenanceInputTokens  int64
	MaintenanceOutputTokens int64
	TotalTokens             int64
	CompactionCycles        int
	SameProcess             bool
	SameSession             bool
	TerminalIdle            bool
	ImplementationHash      string
}

type pipelineOMPActiveRPCSession struct {
	mu            sync.Mutex
	process       *pipelineOMPProcess
	protocol      *pipelineOMPRPCProtocol
	binding       WorkflowContextBridgeBinding
	grantDigest   string
	gitCommitHash string
	specID        string
	projectDir    string
	autoCommit    string
	autoTree      string
	modelScope    string
	allowedModels map[string]struct{}
	sessionID     string
	sequence      int
	closed        bool
	last          pipelineOMPActiveCallReceipt
}

func newPipelineOMPActiveSessionStart(
	backend pipelineOMPBackendConfig,
) pipelineOMPActiveSessionStart {
	return func(ctx context.Context, candidate pipelineOMPManagedActiveCandidate,
		prepared pipelineOMPManagedActivePrepared,
	) (pipelineOMPActivePersistentSession, error) {
		active, err := preparePipelineOMPActiveProcessConfig(backend, candidate, prepared)
		if err != nil {
			return nil, err
		}
		process, err := startPipelineOMPActiveProcess(ctx, active)
		if err != nil {
			return nil, err
		}
		protocol := newPipelineOMPRPCProtocol(process)
		initializeCtx, cancel := context.WithTimeout(ctx, backend.MaxTime)
		defer cancel()
		sessionID, err := protocol.initializeManaged(initializeCtx)
		if err != nil {
			_ = process.Close()
			return nil, err
		}
		return &pipelineOMPActiveRPCSession{
			process: process, protocol: protocol, binding: active.binding,
			grantDigest:   prepared.Binding.GrantDigest,
			gitCommitHash: candidate.Snapshot.GitCommitHash, specID: candidate.Snapshot.SpecID,
			projectDir: filepath.Clean(candidate.Snapshot.ProjectDir), autoCommit: candidate.AutoSourceCommit,
			autoTree: candidate.AutoSourceTree, allowedModels: pipelineOMPActiveModelSet(backend.PhaseModels),
			modelScope: candidate.ModelScopeDigest,
			sessionID:  sessionID,
		}, nil
	}
}

func (session *pipelineOMPActiveRPCSession) Execute(
	ctx context.Context,
	candidate pipelineOMPManagedActiveCandidate,
	prepared pipelineOMPManagedActivePrepared,
) (string, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed || session.process == nil || session.protocol == nil {
		return "", errors.New("pipeline: managed active session is closed")
	}
	selector := candidate.Provider + "/" + candidate.Model
	if prepared.Binding.GrantDigest != session.grantDigest ||
		candidate.Snapshot.GitCommitHash != session.gitCommitHash || candidate.Snapshot.SpecID != session.specID ||
		filepath.Clean(candidate.Snapshot.ProjectDir) != session.projectDir ||
		candidate.AutoSourceCommit != session.autoCommit || candidate.AutoSourceTree != session.autoTree ||
		candidate.ModelScopeDigest != session.modelScope || prepared.Binding.ModelScopeDigest != session.modelScope {
		return "", errors.New("pipeline: managed active run coordinates changed")
	}
	if _, ok := session.allowedModels[selector]; !ok {
		return "", errors.New("pipeline: managed active model escaped the run catalog")
	}
	prompt := candidate.Snapshot.ActivePrompt
	preparePrompt := func() (string, error) {
		if strings.TrimSpace(prompt) == "" || strings.HasPrefix(strings.TrimSpace(prompt), "/auto") ||
			workflowContextRuntimeHash(prompt) != prepared.Binding.DecisionDeltaHash {
			return "", errors.New("pipeline: managed active phase body failed rehydration")
		}
		return prompt, nil
	}
	output, receipt, err := session.protocol.executeManaged(
		ctx, selector, session.binding, session.sessionID, session.sequence > 0, preparePrompt,
	)
	if err != nil {
		_ = session.closeLocked()
		return "", err
	}
	session.sequence++
	receipt.Sequence = session.sequence
	receipt.ImplementationHash = pipelineOMPActiveImplementationDigest()
	session.last = receipt
	return output, nil
}

func pipelineOMPActiveModelSet(models map[pipeline.PhaseID]string) map[string]struct{} {
	result := make(map[string]struct{}, len(models))
	for _, selector := range models {
		result[selector] = struct{}{}
	}
	return result
}

func (session *pipelineOMPActiveRPCSession) Close() error {
	if session == nil {
		return nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.closeLocked()
}

func (session *pipelineOMPActiveRPCSession) closeLocked() error {
	if session.closed {
		return nil
	}
	session.closed = true
	if session.process == nil {
		return nil
	}
	return session.process.Close()
}

func (protocol *pipelineOMPRPCProtocol) initializeManaged(ctx context.Context) (string, error) {
	data, err := protocol.call(ctx, pipelineOMPRPCCommand{
		Type: "negotiate_protocol", ProtocolVersion: pipelineOMPRPCProtocolVersion,
	}, false)
	if err != nil {
		return "", err
	}
	var negotiated struct {
		ProtocolVersion int `json:"protocolVersion"`
	}
	if json.Unmarshal(data, &negotiated) != nil || negotiated.ProtocolVersion != pipelineOMPRPCProtocolVersion {
		return "", errors.New("managed active OMP protocol v2 was not negotiated")
	}
	retry, compaction := false, false
	for _, command := range []pipelineOMPRPCCommand{
		{Type: "set_auto_retry", Enabled: &retry}, {Type: "set_auto_compaction", Enabled: &compaction},
	} {
		if _, err := protocol.call(ctx, command, false); err != nil {
			return "", err
		}
	}
	state, err := protocol.readIdleState(ctx, "managed-start")
	if err != nil || state.AutoCompactionEnabled == nil || *state.AutoCompactionEnabled {
		return "", errors.New("managed active OMP compaction setting is unavailable")
	}
	return state.SessionID, nil
}

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
	rehydrate := func() (string, error) {
		if promptReady {
			return preparedPrompt, nil
		}
		body, prepareErr := preparePrompt()
		if prepareErr != nil {
			return "", prepareErr
		}
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
		TotalTokens:      totalDelta + maintenanceTotal,
		CompactionCycles: boolToPipelineOMPCount(compactBefore),
		SameProcess:      true, SameSession: true, TerminalIdle: true,
	}, nil
}
