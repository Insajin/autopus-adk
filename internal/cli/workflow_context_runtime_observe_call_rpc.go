package cli

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// runCanonicalPrimary executes the full-context arm as exactly one provider
// prompt in a private product-configured OMP RPC process.
func (driver *WorkflowContextManagedRPCDriver) runCanonicalPrimary(
	ctx context.Context,
	prompt string,
) (output string, usage WorkflowContextProviderUsage, runErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	driver.mu.Lock()
	if !driver.bound || driver.running || driver.closed {
		driver.mu.Unlock()
		return "", usage, errors.New("managed OMP RPC driver is not ready")
	}
	binding, options := driver.binding, driver.options
	driver.running = true
	driver.mu.Unlock()
	runCtx, cancel := context.WithTimeout(ctx, options.MaxTime+10*time.Second)
	defer cancel()
	if err := driver.verifyManagedSourceIdentities(); err != nil {
		driver.finishRun(nil)
		return "", usage, err
	}
	if err := verifyWorkflowContextManagedRPCConfig(runCtx, options); err != nil {
		driver.finishRun(nil)
		return "", usage, err
	}
	process, sandboxed, err := startWorkflowContextManagedRPCProcess(runCtx, options, binding)
	if err != nil {
		driver.finishRun(nil)
		return "", usage, err
	}
	protocol := newWorkflowContextManagedRPCProtocol(process.stdin, process.frames, process.done)
	driver.mu.Lock()
	driver.process, driver.protocol = process, protocol
	driver.observation.PID, driver.observation.Sandboxed = process.PID(), sandboxed
	driver.mu.Unlock()
	defer func() {
		runErr = errors.Join(runErr, process.Close())
		driver.finishRun(process)
	}()
	if err := protocol.awaitProductReady(runCtx, options.Prompts[0]); err != nil {
		return "", usage, process.errorWithStderr(err.Error())
	}
	for _, command := range []map[string]any{
		{"id": "observe-protocol", "type": "negotiate_protocol", "protocolVersion": 2},
		{"id": "observe-retry", "type": "set_auto_retry", "enabled": false},
		{"id": "observe-compaction", "type": "set_auto_compaction", "enabled": false},
	} {
		if err := protocol.send(command); err != nil {
			return "", usage, err
		}
		if _, err := protocol.awaitResponse(runCtx, command["id"].(string)); err != nil {
			return "", usage, err
		}
	}
	initial, err := protocol.state(runCtx, "observe-state-before")
	if err != nil || !safeWorkflowContextManagedRPCState(initial) || initial.AutoCompactionEnabled {
		return "", usage, errors.New("full OMP observation initial state is not admission-safe")
	}
	before, err := protocol.sessionStats(runCtx, initial.SessionID, "observe-stats-before")
	if err != nil {
		return "", usage, err
	}
	const promptID = "observe-primary"
	if err := protocol.send(map[string]any{"id": promptID, "type": "prompt", "message": prompt}); err != nil {
		return "", usage, err
	}
	if _, err := protocol.awaitProviderBoundaryState(runCtx, promptID, false); err != nil {
		return "", usage, err
	}
	settled, err := protocol.state(runCtx, "observe-state-after")
	if err != nil || !safeWorkflowContextManagedRPCState(settled) || settled.SessionID != initial.SessionID {
		return "", usage, errors.New("full OMP observation did not settle in the same session")
	}
	after, err := protocol.sessionStats(runCtx, initial.SessionID, "observe-stats-after")
	if err != nil {
		return "", usage, err
	}
	usage, err = workflowContextProviderUsageDelta(before, after)
	if err != nil {
		return "", usage, err
	}
	output, err = protocol.lastAssistantText(runCtx, initial.SessionID)
	if err != nil {
		return "", usage, err
	}
	driver.mu.Lock()
	driver.observation.ProviderTurns = 1
	driver.observation.SameProcess = process.PID() == driver.observation.PID && process.Active()
	driver.observation.SameSession = settled.SessionID == initial.SessionID
	driver.observation.ProviderObserved = true
	driver.mu.Unlock()
	return output, usage, nil
}

func bindWorkflowContextObserveDriver(
	ctx context.Context,
	driver *WorkflowContextManagedRPCDriver,
	taskID string,
) error {
	nonce, err := newWorkflowContextRunNonceHash()
	if err != nil {
		return err
	}
	return driver.Bind(ctx, WorkflowContextBridgeBinding{
		SchemaVersion: workflowContextBridgeSchemaVersion,
		BindingHash:   workflowContextRuntimeHash("observe-binding:" + taskID),
		OptionsHash:   workflowContextRuntimeHash("observe-options:" + taskID),
		SessionHash:   workflowContextRuntimeHash("observe-session:" + taskID),
		NonceHash:     nonce,
	})
}

func validateWorkflowContextObserveOutput(output string, usage WorkflowContextProviderUsage) error {
	if output == "" || usage.PrimaryInputTokens <= 0 || usage.PrimaryOutputTokens <= 0 || usage.TotalTokens <= 0 {
		return fmt.Errorf("OMP observe-call provider result is incomplete")
	}
	return nil
}
