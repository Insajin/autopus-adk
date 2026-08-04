package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: prompt count and byte ceilings keep process-private RPC input below the 1 MiB command envelope.
	workflowContextManagedProductMaxPrompts     = 16
	workflowContextManagedProductMaxPromptBytes = 256 << 10
	workflowContextManagedProductMaxTotalBytes  = (1 << 20) - 4096
	workflowContextManagedDispatchPending       = "pending"
	workflowContextManagedDispatching           = "dispatching"
	workflowContextManagedDispatchCompleted     = "completed"
	workflowContextManagedDispatchFailed        = "failed"
)

type workflowContextManagedRPCProductHooks struct {
	providerTurn  func(int)
	preACK        func()
	nativeStart   func()
	pendingPost   func(string, string)
	manualInitial bool
	observeOnly   bool
}

func workflowContextManagedRPCProductMode(options WorkflowContextManagedRPCOptions) (bool, error) {
	if options.ProjectDir == "" && options.Prompts == nil {
		return false, nil
	}
	if strings.TrimSpace(options.ProjectDir) == "" || len(options.Prompts) == 0 {
		return false, errors.New("managed OMP product mode is ambiguous")
	}
	return true, nil
}

func validateWorkflowContextManagedProductPrompts(prompts []string) error {
	if len(prompts) != 2 || len(prompts) > workflowContextManagedProductMaxPrompts {
		return errors.New("managed OMP product prompt count is invalid")
	}
	total := 0
	for _, prompt := range prompts {
		if strings.TrimSpace(prompt) == "" || len(prompt) > workflowContextManagedProductMaxPromptBytes {
			return errors.New("managed OMP product prompt is invalid")
		}
		total += len(prompt)
		if total > workflowContextManagedProductMaxTotalBytes {
			return errors.New("managed OMP product prompts exceed the input limit")
		}
	}
	first := prompts[0]
	validAuto := first == "/auto" ||
		(strings.HasPrefix(first, "/auto ") && strings.TrimSpace(first[len("/auto "):]) != "") ||
		(strings.HasPrefix(first, "/auto-") && len(strings.Fields(first)[0]) > len("/auto-"))
	if !validAuto {
		return errors.New("managed OMP first product prompt must be an exact /auto command")
	}
	return nil
}

func validateWorkflowContextManagedProductPromptsForOptions(options WorkflowContextManagedRPCOptions) error {
	if !options.ObserveOnly {
		return validateWorkflowContextManagedProductPrompts(options.Prompts)
	}
	if len(options.Prompts) != 2 {
		return errors.New("managed OMP observation setup prompt count is invalid")
	}
	for _, prompt := range options.Prompts {
		if strings.TrimSpace(prompt) == "" || len(prompt) > workflowContextManagedProductMaxPromptBytes ||
			strings.HasPrefix(strings.TrimSpace(prompt), "/auto") ||
			!strings.Contains(prompt, "AUTOPUS_PROVIDER_PHASE=setup") {
			return errors.New("managed OMP observation setup prompt is unsafe")
		}
	}
	return nil
}

func runWorkflowContextManagedRPCProduct(
	ctx context.Context,
	protocol *workflowContextManagedRPCProtocol,
	binding WorkflowContextBridgeBinding,
	sessionID string,
	prompts []string,
	emit func(WorkflowContextRuntimeEvent) error,
) error {
	return runWorkflowContextManagedRPCProductWithHooks(
		ctx, protocol, binding, sessionID, prompts, emit, workflowContextManagedRPCProductHooks{}, 1,
	)
}

func (driver *WorkflowContextManagedRPCDriver) runProduct(
	ctx context.Context,
	emit func(WorkflowContextRuntimeEvent) error,
	sessionID string,
	prompts []string,
) error {
	driver.mu.Lock()
	cycles := driver.options.CompactionCycles
	driver.mu.Unlock()
	hooks := workflowContextManagedRPCProductHooks{
		providerTurn: driver.setProviderTurns,
		preACK:       driver.notePreACK,
		nativeStart:  driver.noteNativeStart,
		pendingPost: func(id, session string) {
			if driver.options.InitialManualCompaction {
				driver.mu.Lock()
				driver.protocolManual = true
				driver.mu.Unlock()
			}
			driver.setPendingWorkflowContextManagedDispatch(id, session)
		},
		manualInitial: driver.options.InitialManualCompaction,
		observeOnly:   driver.options.ObserveOnly,
	}
	return runWorkflowContextManagedRPCProductWithHooks(
		ctx, driver.protocol, driver.binding, sessionID, prompts,
		func(event WorkflowContextRuntimeEvent) error {
			if event.Kind == WorkflowContextEventCompacted {
				event.HistoryAfterTokens = driver.historyAfterTokens()
			}
			return emit(event)
		}, hooks, cycles,
	)
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: shared product RPC lifecycle used by the driver and protocol-level verification.
// @AX:REASON [AUTO]: driver and protocol tests depend on the exact prompt, native compaction, and supervisor event sequence.
// @AX:WARN [AUTO]: this protocol state machine has more than 15 fail-closed branches.
// @AX:REASON [AUTO]: prompt order, provider turns, native lifecycle, and PRE/POST ACK authority must advance without bypass.
func runWorkflowContextManagedRPCProductWithHooks(
	ctx context.Context,
	protocol *workflowContextManagedRPCProtocol,
	binding WorkflowContextBridgeBinding,
	sessionID string,
	prompts []string,
	emit func(WorkflowContextRuntimeEvent) error,
	hooks workflowContextManagedRPCProductHooks,
	maxCycles int,
) error {
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: the eight-cycle ceiling bounds replay state and one live RPC stream.
	if protocol == nil || emit == nil || strings.TrimSpace(sessionID) == "" || maxCycles < 1 || maxCycles > 8 {
		return errors.New("managed OMP product protocol input is invalid")
	}
	if err := validateWorkflowContextManagedBinding(binding); err != nil {
		return err
	}
	if err := validateWorkflowContextManagedProductPromptsForOptions(WorkflowContextManagedRPCOptions{
		Prompts: prompts, ObserveOnly: hooks.observeOnly,
	}); err != nil {
		return err
	}
	prompts = append([]string(nil), prompts...)
	if err := sendWorkflowContextManagedProductPrompt(protocol, 0, prompts[0]); err != nil {
		return err
	}

	sent, accepted, started, turns, agentEnds := 1, 0, 0, 0, 0
	nativeStarted, preACKed, completedCycles, manualRequested := false, false, 0, false
	for {
		frame, err := protocol.next(ctx)
		if err != nil {
			return fmt.Errorf("managed OMP product stream ended: %w", err)
		}
		if frame.Type == "extension_error" {
			return errors.New("managed OMP context bridge extension failed")
		}
		switch frame.Type {
		case "response":
			if frame.Command == "compact" {
				wantID := fmt.Sprintf("managed-compact-%d", completedCycles+1)
				if (!manualRequested && completedCycles == 0) || frame.ID != wantID || frame.Success == nil || !*frame.Success {
					return fmt.Errorf("managed OMP manual compaction response is invalid: id=%s want=%s success=%v",
						frame.ID, wantID, frame.Success != nil && *frame.Success)
				}
				continue
			}
			wantID := fmt.Sprintf("managed-product-prompt-%d", turns+1)
			if nativeStarted || accepted != turns || frame.ID != wantID || frame.Command != "prompt" ||
				frame.Success == nil || !*frame.Success {
				return errors.New("managed OMP product prompt response is invalid")
			}
			accepted++
		case "agent_start":
			if nativeStarted || accepted != turns+1 || started != turns {
				return errors.New("managed OMP product agent start is out of order")
			}
			started++
		case "turn_end":
			if nativeStarted || accepted != turns+1 || started != turns+1 || turns >= sent {
				return errors.New("managed OMP product turn is out of order")
			}
			turns++
			if hooks.providerTurn != nil {
				hooks.providerTurn(turns)
			}
		case "agent_end":
			if nativeStarted || agentEnds >= turns || turns != sent || started != turns || accepted != turns ||
				frame.IsTerminal != nil && !*frame.IsTerminal {
				return errors.New("managed OMP product agent end is out of order")
			}
			agentEnds++
			if sent < len(prompts) {
				if err := sendWorkflowContextManagedProductPrompt(protocol, sent, prompts[sent]); err != nil {
					return err
				}
				sent++
			} else if hooks.manualInitial && completedCycles == 0 && !manualRequested {
				if err := protocol.send(map[string]any{
					"id": "managed-compact-1", "type": "compact",
					"customInstructions": "AUTOPUS_PROVIDER_PHASE=compaction",
				}); err != nil {
					return err
				}
				manualRequested = true
			}
		case "auto_compaction_start":
			initialIncomplete := completedCycles == 0 && (sent != len(prompts) || accepted != len(prompts) ||
				started != len(prompts) || turns != len(prompts) || agentEnds < len(prompts)-1)
			validReason := frame.Reason == "threshold" || (completedCycles > 0 || manualRequested) && frame.Reason == "manual"
			if nativeStarted || initialIncomplete || !validReason || frame.Action != "snapcompact" {
				return fmt.Errorf(
					"managed OMP product native compaction start is invalid: sent=%d turns=%d agent_ends=%d",
					sent, turns, agentEnds,
				)
			}
			nativeStarted = true
			if hooks.nativeStart != nil {
				hooks.nativeStart()
			}
		case "extension_ui_request":
			if workflowContextManagedProductNotification(frame.Method) {
				continue
			}
			if frame.Method != "confirm" {
				return fmt.Errorf("managed OMP product emitted an unsupported UI request: method=%s", frame.Method)
			}
			event, bridgeErr := protocol.bridgeRequest(frame, binding)
			if bridgeErr != nil {
				return bridgeErr
			}
			if event == WorkflowContextEventPreCompaction {
				manualPreStart := (completedCycles > 0 || manualRequested) && !nativeStarted
				if (!nativeStarted && !manualPreStart) || preACKed {
					return errors.New("managed OMP product pre-compaction ACK is out of order")
				}
				if err := emit(WorkflowContextRuntimeEvent{Kind: WorkflowContextEventPreCompaction}); err != nil {
					return err
				}
				if err := protocol.confirm(frame.ID); err != nil {
					return err
				}
				preACKed = true
				if manualPreStart {
					nativeStarted = true
					if hooks.nativeStart != nil {
						hooks.nativeStart()
					}
				}
				if hooks.preACK != nil {
					hooks.preACK()
				}
				continue
			}
			if !nativeStarted || !preACKed {
				return errors.New("managed OMP product post-compaction hook is out of order")
			}
			if hooks.pendingPost != nil {
				hooks.pendingPost(frame.ID, sessionID)
			}
			if err := emit(WorkflowContextRuntimeEvent{Kind: WorkflowContextEventCompacted}); err != nil {
				return err
			}
			if err := emit(WorkflowContextRuntimeEvent{Kind: WorkflowContextEventPostCompaction}); err != nil {
				return err
			}
			completedCycles++
			manualRequested = false
			if completedCycles == maxCycles {
				return nil
			}
			nativeStarted, preACKed = false, false
		case "auto_compaction_end":
			return errors.New("managed OMP product native completion bypassed the post-compaction ACK barrier")
		}
	}
}
