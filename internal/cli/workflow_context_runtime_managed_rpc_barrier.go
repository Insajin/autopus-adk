package cli

import (
	"context"
	"errors"
	"fmt"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: the exact pause ACK is defense in depth; ordered lifecycle correlation remains authoritative when project settings keep readback enabled.
func (protocol *workflowContextManagedRPCProtocol) requestCompactionPause(
	ctx context.Context, id string,
) error {
	if err := protocol.send(map[string]any{
		"id": id, "type": "set_auto_compaction", "enabled": false,
	}); err != nil {
		return err
	}
	response, err := protocol.awaitResponse(ctx, id)
	if err != nil {
		return fmt.Errorf("await managed OMP compaction pause: %w", err)
	}
	if response.Command != "set_auto_compaction" {
		return errors.New("managed OMP compaction pause response is invalid")
	}
	return nil
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: shared idle-state contract for startup, cycle triggering, admission, and post-compaction readback.
// @AX:REASON [AUTO]: four production callers depend on the same session, streaming, compaction, queue, and message-count invariants.
func safeWorkflowContextManagedRPCState(state workflowContextManagedRPCState) bool {
	return state.SessionID != "" && !state.IsStreaming && !state.IsCompacting &&
		state.QueuedMessageCount == 0 && state.MessageCount >= 0
}

func (driver *WorkflowContextManagedRPCDriver) wantsNextCompactionCycle() bool {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	return driver.options.CompactionCycles > 1 && driver.dispatchSequence < driver.options.CompactionCycles
}

func (driver *WorkflowContextManagedRPCDriver) admitManagedCanonicalMessage(
	ctx context.Context, lease workflowContextManagedDispatchLease, message string, beforeMessages int,
) (bool, string, WorkflowContextProviderUsage, error) {
	usage := WorkflowContextProviderUsage{}
	before := workflowContextManagedRPCSessionStats{}
	var err error
	if driver.options.CaptureStats {
		before, err = lease.protocol.sessionStats(ctx, lease.initialSession, "managed-stats-before-primary")
		if err != nil {
			return false, "", usage, err
		}
	}
	if err := lease.protocol.send(map[string]any{
		"id": lease.admissionID, "type": "prompt", "message": message,
	}); err != nil {
		return false, "", usage, err
	}
	nativeStarted, err := lease.protocol.awaitProviderBoundaryState(
		ctx, lease.admissionID, driver.wantsNextCompactionCycle(),
	)
	if err != nil || nativeStarted {
		return nativeStarted, "", usage, err
	}
	state, err := lease.protocol.state(ctx, "managed-state-admitted")
	if err != nil || !safeWorkflowContextManagedRPCState(state) ||
		state.SessionID != lease.initialSession || state.MessageCount <= beforeMessages {
		return false, "", usage, errors.New("managed OMP admitted provider state is not session-bound")
	}
	output := ""
	if driver.options.CaptureOutput {
		output, err = lease.protocol.lastAssistantText(ctx, lease.initialSession)
		if err != nil {
			return false, "", usage, err
		}
	}
	if driver.options.CaptureStats {
		after, statsErr := lease.protocol.sessionStats(ctx, lease.initialSession, "managed-stats-after-primary")
		if statsErr != nil {
			return false, "", usage, statsErr
		}
		usage, err = workflowContextProviderUsageDelta(before, after)
		if err != nil {
			return false, "", usage, err
		}
	}
	triggered, err := driver.triggerNextWorkflowContextManagedCompaction(ctx, lease, message, state.MessageCount)
	return triggered, output, usage, err
}
