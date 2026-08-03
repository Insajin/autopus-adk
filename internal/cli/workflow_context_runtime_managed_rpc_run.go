package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: 320-repeat seed and threshold prompts are bounded stimuli for crossing the live compaction threshold within two provider turns.
func (driver *WorkflowContextManagedRPCDriver) Run(
	ctx context.Context, emit func(WorkflowContextRuntimeEvent) error,
) (runErr error) {
	driver.mu.Lock()
	if !driver.bound || driver.running || driver.closed {
		driver.mu.Unlock()
		return errors.New("managed OMP RPC driver is not ready")
	}
	binding, options := driver.binding, driver.options
	driver.running = true
	driver.mu.Unlock()
	if err := driver.verifyManagedSourceIdentities(); err != nil {
		driver.finishRun(nil)
		return err
	}
	if err := verifyWorkflowContextManagedRPCConfig(ctx, options); err != nil {
		driver.finishRun(nil)
		return err
	}
	if err := driver.verifyManagedSourceIdentities(); err != nil {
		driver.finishRun(nil)
		return err
	}
	process, sandboxed, err := startWorkflowContextManagedRPCProcess(ctx, options, binding)
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

	if err := protocol.awaitReady(ctx); err != nil {
		return process.errorWithStderr(err.Error())
	}
	for _, command := range []map[string]any{
		{"id": "managed-protocol", "type": "negotiate_protocol", "protocolVersion": 2},
		{"id": "managed-retry", "type": "set_auto_retry", "enabled": false},
		{"id": "managed-compaction", "type": "set_auto_compaction", "enabled": true},
	} {
		if err := protocol.send(command); err != nil {
			return err
		}
		if _, err := protocol.awaitResponse(ctx, command["id"].(string)); err != nil {
			return err
		}
	}
	initial, err := protocol.state(ctx, "managed-state-before")
	if err != nil || !safeWorkflowContextManagedRPCState(initial) {
		return errors.New("managed OMP initial state is not admission-safe")
	}
	if err := protocol.send(map[string]any{
		"id": "managed-seed", "type": "prompt", "message": strings.Repeat("bounded seed context ", 320),
	}); err != nil {
		return err
	}
	return driver.runCompaction(ctx, emit, initial.SessionID)
}

// @AX:WARN [AUTO]: managed compaction sequencing contains 15 if branches.
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
			driver.mu.Lock()
			driver.protocolPostID = frame.ID
			driver.protocolSessionID = sessionID
			driver.mu.Unlock()
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

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: a 5-second probe and 16 KiB read cap bound the preflight configuration readback.
func verifyWorkflowContextManagedRPCConfig(
	ctx context.Context, options WorkflowContextManagedRPCOptions,
) error {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, options.Executable,
		"config", "get", "compaction.autoContinue", "--json")
	cmd.Dir = options.Workspace
	cmd.Env = append([]string(nil), options.Environment...)
	if _, err := configureWorkflowContextManagedRPCSandbox(cmd, options.AllowedEndpoint); err != nil {
		return err
	}
	output, err := processprobe.OutputLimited(cmd, 16<<10)
	if err != nil {
		return fmt.Errorf("read managed OMP autoContinue config: %w", err)
	}
	var readback struct {
		Key   string `json:"key"`
		Value any    `json:"value"`
	}
	if json.Unmarshal(output, &readback) != nil || readback.Key != "compaction.autoContinue" || readback.Value != false {
		return errors.New("managed OMP autoContinue must read back false")
	}
	return nil
}

func (driver *WorkflowContextManagedRPCDriver) Dispatch(
	ctx context.Context, dispatch WorkflowContextDispatch,
) (WorkflowContextDispatchAck, error) {
	driver.mu.Lock()
	protocol, process := driver.protocol, driver.process
	postID, initialSession := driver.protocolPostID, driver.protocolSessionID
	binding, running := driver.binding, driver.running
	driver.mu.Unlock()
	if !running || protocol == nil || process == nil || !process.Active() || postID == "" {
		return WorkflowContextDispatchAck{}, errors.New("managed OMP dispatch is outside the live post hook")
	}
	message, err := buildWorkflowContextManagedAdmission(dispatch)
	if err != nil {
		return WorkflowContextDispatchAck{}, err
	}
	if err := protocol.confirm(postID); err != nil {
		return WorkflowContextDispatchAck{}, err
	}
	if err := protocol.awaitNativeCompactionEnd(ctx); err != nil {
		return WorkflowContextDispatchAck{}, err
	}
	driver.noteNativeEnd()
	driver.notePostACK()
	state, err := protocol.state(ctx, "managed-state-after")
	if err != nil || !safeWorkflowContextManagedRPCState(state) || state.SessionID != initialSession {
		return WorkflowContextDispatchAck{}, errors.New("managed OMP post-compaction state is not admission-safe")
	}
	if err := protocol.send(map[string]any{
		"id": "managed-admission", "type": "prompt", "message": message,
	}); err != nil {
		return WorkflowContextDispatchAck{}, err
	}
	if err := protocol.awaitProviderBoundary(ctx, "managed-admission"); err != nil {
		return WorkflowContextDispatchAck{}, err
	}
	driver.mu.Lock()
	driver.observation.ProviderTurns++
	driver.observation.SameProcess = process.PID() == driver.observation.PID && process.Active()
	driver.observation.SameSession = state.SessionID == initialSession
	driver.observation.ProviderObserved = true
	driver.mu.Unlock()
	return WorkflowContextDispatchAck{
		SchemaVersion: workflowContextDispatchAckSchemaVersion,
		BindingHash:   binding.BindingHash, OptionsHash: binding.OptionsHash,
		SessionHash: binding.SessionHash, NonceHash: binding.NonceHash, ProviderObserved: true,
	}, nil
}

func safeWorkflowContextManagedRPCState(state workflowContextManagedRPCState) bool {
	return state.SessionID != "" && !state.IsStreaming && !state.IsCompacting &&
		state.QueuedMessageCount == 0 && state.AutoCompactionEnabled
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
