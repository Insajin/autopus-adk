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
	PreCompactionACKs       int
	PostCompactionACKs      int
	CanonicalReadmissions   int
	EphemeralReadmissions   int
	SameProcess             bool
	SameSession             bool
	TerminalIdle            bool
	BridgeBindingHash       string
	SessionBindingHash      string
	ContextBindingHash      string
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
	compactions   int
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
	compactBefore := session.sequence > 0
	output, receipt, err := session.protocol.executeManaged(
		ctx, selector, session.binding, session.sessionID, compactBefore, preparePrompt,
	)
	if err != nil {
		_ = session.closeLocked()
		return "", err
	}
	receipt.ContextBindingHash = prepared.Binding.BindingHash
	expectedCompactions := receipt.CompactionCycles
	if expectedCompactions < 0 || expectedCompactions > 1 ||
		!compactBefore && expectedCompactions != 0 ||
		receipt.PreCompactionACKs != expectedCompactions ||
		receipt.PostCompactionACKs != expectedCompactions ||
		receipt.CanonicalReadmissions != expectedCompactions ||
		receipt.EphemeralReadmissions != expectedCompactions ||
		receipt.SessionBindingHash != workflowContextRuntimeHash(session.sessionID+"\x00"+session.binding.NonceHash) ||
		receipt.ContextBindingHash == "" || !receipt.SameProcess || !receipt.SameSession || !receipt.TerminalIdle {
		_ = session.closeLocked()
		return "", errors.New("pipeline: managed active compaction admission evidence is incomplete")
	}
	session.sequence++
	session.compactions += receipt.CompactionCycles
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
	var evidenceErr error
	maxCompactions := max(session.sequence-1, 0)
	if session.compactions < 0 || session.compactions > maxCompactions {
		evidenceErr = errors.New("pipeline: managed active multi-compaction session gate failed")
	}
	if session.process == nil {
		return evidenceErr
	}
	return errors.Join(evidenceErr, session.process.Close())
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
