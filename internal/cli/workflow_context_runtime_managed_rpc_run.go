package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: managed driver entrypoint selects the authoritative product path or the bounded canary path.
// @AX:REASON [AUTO]: the supervisor interface depends on this lifecycle; the canary branch uses 320-repeat prompts to cross the threshold within two turns.
// @AX:WARN [AUTO]: managed driver startup has cyclomatic complexity 19.
// @AX:REASON [AUTO]: gocyclo reports 19 across identity, effective configuration, process startup, discovery, negotiation, and initial state gates.
func (driver *WorkflowContextManagedRPCDriver) Run(
	ctx context.Context, emit func(WorkflowContextRuntimeEvent) error,
) (runErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	driver.mu.Lock()
	if !driver.bound || driver.running || driver.closed {
		driver.mu.Unlock()
		return errors.New("managed OMP RPC driver is not ready")
	}
	binding, options := driver.binding, driver.options
	driver.running = true
	driver.mu.Unlock()
	runCtx, cancelRun := context.WithTimeout(ctx, options.MaxTime+10*time.Second)
	defer cancelRun()
	if err := driver.verifyManagedSourceIdentities(); err != nil {
		driver.finishRun(nil)
		return err
	}
	if err := verifyWorkflowContextManagedRPCConfig(runCtx, options); err != nil {
		driver.finishRun(nil)
		return err
	}
	if err := driver.verifyManagedSourceIdentities(); err != nil {
		driver.finishRun(nil)
		return err
	}
	process, sandboxed, err := startWorkflowContextManagedRPCProcess(runCtx, options, binding)
	if err != nil {
		driver.finishRun(nil)
		return err
	}
	protocol := newWorkflowContextManagedRPCProtocol(process.stdin, process.frames, process.done)
	driver.mu.Lock()
	driver.process, driver.protocol = process, protocol
	driver.observation.PID = process.PID()
	driver.observation.Sandboxed = sandboxed
	driver.mu.Unlock()
	defer func() {
		closeErr := process.Close()
		driver.finishRun(process)
		runErr = errors.Join(runErr, closeErr)
	}()

	var readyErr error
	if len(options.Prompts) != 0 && !options.ObserveOnly {
		readyErr = protocol.awaitProductReady(runCtx, options.Prompts[0])
	} else {
		readyErr = protocol.awaitReady(runCtx)
	}
	if readyErr != nil {
		return process.errorWithStderr(readyErr.Error())
	}
	for _, command := range []map[string]any{
		{"id": "managed-protocol", "type": "negotiate_protocol", "protocolVersion": 2},
		{"id": "managed-retry", "type": "set_auto_retry", "enabled": false},
		{"id": "managed-compaction", "type": "set_auto_compaction", "enabled": true},
	} {
		if err := protocol.send(command); err != nil {
			return err
		}
		if _, err := protocol.awaitResponse(runCtx, command["id"].(string)); err != nil {
			return err
		}
	}
	initial, err := protocol.state(runCtx, "managed-state-before")
	if err != nil || !safeWorkflowContextManagedRPCState(initial) || !initial.AutoCompactionEnabled {
		return fmt.Errorf(
			"managed OMP initial state is not admission-safe: streaming=%t compacting=%t queued=%d auto=%t",
			initial.IsStreaming, initial.IsCompacting, initial.QueuedMessageCount, initial.AutoCompactionEnabled,
		)
	}
	if len(options.Prompts) != 0 {
		return driver.runProduct(runCtx, emit, initial.SessionID, options.Prompts)
	}
	if err := protocol.send(map[string]any{
		"id": "managed-seed", "type": "prompt", "message": strings.Repeat("bounded seed context ", 320),
	}); err != nil {
		return err
	}
	return driver.runCompaction(runCtx, emit, initial.SessionID)
}

// @AX:WARN [AUTO]: managed compaction sequencing has cyclomatic complexity 29 and 16 fail-closed if branches.
// @AX:REASON [AUTO]: provider-turn budget, native start, pre/post ACK order, bridge validation, and supervisor events form one fail-closed state machine.
func (driver *WorkflowContextManagedRPCDriver) runCompaction(
	ctx context.Context, emit func(WorkflowContextRuntimeEvent) error, sessionID string,
) error {
	thresholdSent, preACKed, nativeStarted := false, false, false
	providerTurns := 0
	for {
		frame, err := driver.protocol.next(ctx)
		if err != nil {
			return fmt.Errorf("managed OMP compaction stream ended: %w", err)
		}
		if frame.Type == "extension_error" {
			return errors.New("managed OMP context bridge extension failed")
		}
		switch frame.Type {
		case "turn_end":
			providerTurns++
			driver.setProviderTurns(providerTurns)
			if providerTurns > 2 {
				return fmt.Errorf("managed OMP provider turn budget exceeded: %d", providerTurns)
			}
		case "agent_end":
			if providerTurns == 1 && !thresholdSent {
				if err := driver.protocol.send(map[string]any{
					"id": "managed-threshold", "type": "prompt",
					"message": strings.Repeat("bounded threshold context ", 320),
				}); err != nil {
					return err
				}
				thresholdSent = true
			}
		case "extension_ui_request":
			if frame.Method != "confirm" {
				continue
			}
			event, bridgeErr := driver.protocol.bridgeRequest(frame, driver.binding)
			if bridgeErr != nil {
				return bridgeErr
			}
			if event == WorkflowContextEventPreCompaction {
				if preACKed || !nativeStarted || !thresholdSent || providerTurns != 2 {
					return errors.New("managed OMP pre-compaction ACK is out of order")
				}
				if err := emit(WorkflowContextRuntimeEvent{Kind: WorkflowContextEventPreCompaction}); err != nil {
					return err
				}
				if err := driver.protocol.confirm(frame.ID); err != nil {
					return err
				}
				preACKed = true
				driver.notePreACK()
				continue
			}
			if !nativeStarted || !preACKed {
				return errors.New("managed OMP post-compaction hook is out of order")
			}
			driver.setPendingWorkflowContextManagedDispatch(frame.ID, sessionID)
			if err := emit(WorkflowContextRuntimeEvent{
				Kind: WorkflowContextEventCompacted, HistoryAfterTokens: driver.historyAfterTokens(),
			}); err != nil {
				return err
			}
			if err := emit(WorkflowContextRuntimeEvent{Kind: WorkflowContextEventPostCompaction}); err != nil {
				return err
			}
			return nil
		case "auto_compaction_start":
			if nativeStarted || frame.Reason != "threshold" || frame.Action != "snapcompact" {
				return errors.New("managed OMP native compaction start is invalid")
			}
			nativeStarted = true
			driver.noteNativeStart()
		case "auto_compaction_end":
			return errors.New("managed OMP native completion bypassed the post-compaction ACK barrier")
		}
	}
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: supervisor dispatch boundary for canonical provider admission in the live OMP session.
// @AX:REASON [AUTO]: the managed-driver interface and multi-cycle runtime depend on barrier, state, provider, and ACK evidence remaining coupled.
// @AX:WARN [AUTO]: post-compaction dispatch has cyclomatic complexity 15 and 11 fail-closed if branches.
// @AX:REASON [AUTO]: native completion, compaction barrier, state readback, provider observation, and next-cycle trigger must remain ordered.
func (driver *WorkflowContextManagedRPCDriver) Dispatch(
	ctx context.Context, dispatch WorkflowContextDispatch,
) (WorkflowContextDispatchAck, error) {
	lease, err := driver.beginWorkflowContextManagedDispatch()
	if err != nil {
		return WorkflowContextDispatchAck{}, err
	}
	if !lease.process.Active() {
		return WorkflowContextDispatchAck{}, errors.New("managed OMP dispatch is outside the live post hook")
	}
	succeeded := false
	defer func() { driver.finishWorkflowContextManagedDispatch(succeeded) }()
	message, err := buildWorkflowContextManagedAdmission(dispatch)
	if err != nil {
		return WorkflowContextDispatchAck{}, err
	}
	if err := lease.protocol.confirm(lease.postID); err != nil {
		return WorkflowContextDispatchAck{}, err
	}
	if lease.manual {
		if err := lease.protocol.awaitManualCompactionCompletion(ctx, lease.compactID); err != nil {
			return WorkflowContextDispatchAck{}, err
		}
	} else {
		if err := lease.protocol.awaitNativeCompactionEnd(ctx); err != nil {
			return WorkflowContextDispatchAck{}, err
		}
	}
	if err := lease.protocol.requestCompactionPause(ctx, lease.barrierID); err != nil {
		return WorkflowContextDispatchAck{}, err
	}
	state, err := lease.protocol.state(ctx, "managed-state-after")
	if err != nil || !safeWorkflowContextManagedRPCState(state) || state.SessionID != lease.initialSession {
		return WorkflowContextDispatchAck{}, fmt.Errorf("managed OMP post-compaction state is not admission-safe: err=%v streaming=%t compacting=%t queued=%d auto=%t session_match=%t", err, state.IsStreaming, state.IsCompacting, state.QueuedMessageCount, state.AutoCompactionEnabled, state.SessionID == lease.initialSession)
	}
	driver.noteNativeEnd()
	driver.notePostACK()
	triggered, output, usage, err := driver.admitManagedCanonicalMessage(ctx, lease, message, state.MessageCount)
	if err != nil {
		return WorkflowContextDispatchAck{}, err
	}
	driver.mu.Lock()
	driver.observation.ProviderTurns++
	if triggered {
		driver.observation.ProviderTurns++
	}
	driver.observation.SameProcess = lease.process.PID() == driver.observation.PID && lease.process.Active()
	driver.observation.SameSession = state.SessionID == lease.initialSession
	driver.observation.ProviderObserved = true
	driver.mu.Unlock()
	succeeded = true
	return WorkflowContextDispatchAck{
		SchemaVersion: workflowContextDispatchAckSchemaVersion,
		BindingHash:   lease.binding.BindingHash, OptionsHash: lease.binding.OptionsHash,
		SessionHash: lease.binding.SessionHash, NonceHash: lease.binding.NonceHash, ProviderObserved: true,
		providerOutput: output, providerUsage: usage,
	}, nil
}
func (driver *WorkflowContextManagedRPCDriver) historyAfterTokens() map[string]int {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	result := make(map[string]int, len(driver.options.HistoryAfterTokens))
	for id, tokens := range driver.options.HistoryAfterTokens {
		result[id] = tokens
	}
	return result
}

func (driver *WorkflowContextManagedRPCDriver) finishRun(process *workflowContextManagedRPCProcess) {
	driver.mu.Lock()
	driver.running = false
	if process != nil {
		driver.observation.ProcessActiveAfterCleanup = process.Active()
	}
	driver.mu.Unlock()
}

func (driver *WorkflowContextManagedRPCDriver) setProviderTurns(value int) {
	driver.mu.Lock()
	driver.observation.ProviderTurns = value
	driver.mu.Unlock()
}

func (driver *WorkflowContextManagedRPCDriver) notePreACK() {
	driver.mu.Lock()
	driver.observation.PreACKs++
	driver.mu.Unlock()
}

func (driver *WorkflowContextManagedRPCDriver) notePostACK() {
	driver.mu.Lock()
	driver.observation.PostACKs++
	driver.observation.CanonicalReadmissions++
	driver.observation.EphemeralReadmissions++
	driver.mu.Unlock()
}

func (driver *WorkflowContextManagedRPCDriver) noteNativeStart() {
	driver.mu.Lock()
	driver.observation.NativeStarts++
	driver.mu.Unlock()
}

func (driver *WorkflowContextManagedRPCDriver) noteNativeEnd() {
	driver.mu.Lock()
	driver.observation.NativeEnds++
	driver.mu.Unlock()
}
