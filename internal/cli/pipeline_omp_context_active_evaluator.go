package cli

import (
	"context"
	"errors"
	"strings"
)

type pipelineOMPActiveSandboxMode uint8

const (
	pipelineOMPActiveSandboxManaged pipelineOMPActiveSandboxMode = iota
	pipelineOMPActiveSandboxInheritedParent
)

// pipelineOMPActiveEvaluatorSession uses the production process and protocol
// implementation without consuming promotion authority. Only the explicit-live
// observe-session command may construct it.
type pipelineOMPActiveEvaluatorSession struct {
	process   *pipelineOMPProcess
	protocol  *pipelineOMPRPCProtocol
	binding   WorkflowContextBridgeBinding
	selector  string
	sessionID string
	optimized bool
	sequence  int
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: evaluator startup is the shared full and optimized live-session boundary.
// @AX:REASON [AUTO]: observe-session and RPC integration callers depend on identical process, protocol, binding, and sandbox-mode construction.
func startPipelineOMPActiveEvaluatorSession(
	ctx context.Context,
	backend pipelineOMPBackendConfig,
	candidate pipelineOMPManagedActiveCandidate,
	prepared pipelineOMPManagedActivePrepared,
	optimized bool,
	sandboxMode pipelineOMPActiveSandboxMode,
) (*pipelineOMPActiveEvaluatorSession, error) {
	active, err := preparePipelineOMPActiveProcessConfig(backend, candidate, prepared)
	if err != nil {
		return nil, err
	}
	active.sandboxMode = sandboxMode
	process, err := startPipelineOMPActiveProcess(ctx, active)
	if err != nil {
		return nil, err
	}
	protocol := newPipelineOMPRPCProtocol(process)
	sessionID, err := protocol.initializeManaged(ctx)
	if err != nil {
		_ = process.Close()
		return nil, err
	}
	return &pipelineOMPActiveEvaluatorSession{
		process: process, protocol: protocol, binding: active.binding,
		selector:  candidate.Provider + "/" + candidate.Model,
		sessionID: sessionID, optimized: optimized,
	}, nil
}

func (session *pipelineOMPActiveEvaluatorSession) Execute(
	ctx context.Context,
	prompt string,
) (string, pipelineOMPActiveCallReceipt, error) {
	if session == nil || session.process == nil || session.protocol == nil ||
		strings.TrimSpace(prompt) == "" || strings.HasPrefix(strings.TrimSpace(prompt), "/auto") {
		return "", pipelineOMPActiveCallReceipt{}, errors.New("pipeline: active evaluator input is invalid")
	}
	output, receipt, err := session.protocol.executeManaged(
		ctx, session.selector, session.binding, session.sessionID,
		session.optimized && session.sequence > 0,
		func() (string, error) { return prompt, nil },
	)
	if err != nil {
		_ = session.Close()
		return "", pipelineOMPActiveCallReceipt{}, err
	}
	session.sequence++
	receipt.Sequence = session.sequence
	receipt.ImplementationHash = pipelineOMPActiveImplementationDigest()
	return output, receipt, nil
}

func (session *pipelineOMPActiveEvaluatorSession) Close() error {
	if session == nil || session.process == nil {
		return nil
	}
	err := session.process.Close()
	session.process, session.protocol = nil, nil
	return err
}

func (session *pipelineOMPActiveEvaluatorSession) PID() int {
	if session == nil || session.process == nil || session.process.cmd == nil || session.process.cmd.Process == nil {
		return 0
	}
	return session.process.cmd.Process.Pid
}
