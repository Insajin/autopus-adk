package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

// @AX:WARN [AUTO]: runtime request validation contains 8 if branches.
// @AX:REASON [AUTO]: identity, task/session metadata, capabilities, policy, promotion, binding, and receipt root are fail-closed.
func validateWorkflowContextRuntimeRequest(request WorkflowContextRuntimeRequest) error {
	if request.Policy.HistoryMode != config.OMPContextHistoryActive {
		return fmt.Errorf("OMP context runtime requires explicit active history mode")
	}
	if request.Policy.MemoryMode != config.OMPContextMemoryOff && request.Policy.MemoryMode != config.OMPContextMemoryShadow {
		return fmt.Errorf("OMP context runtime forbids active memory injection")
	}
	if request.Policy.CapabilityPolicy != config.OMPContextCapabilityProbeRequired {
		return fmt.Errorf("OMP context runtime requires capability proof")
	}
	if request.Policy.MutationScope != config.OMPContextMutationSessionOverlay {
		return fmt.Errorf("OMP context runtime requires a session overlay")
	}
	if request.Policy.RuntimeRootPolicy != request.RootClass {
		return fmt.Errorf("OMP context runtime root class disagrees with effective policy")
	}
	if request.RootClass != config.OMPContextRuntimeNoSession && request.RootClass != config.OMPContextRuntimeIsolatedTaskOwned {
		return fmt.Errorf("OMP context runtime root is not task-owned")
	}
	if request.Policy.HistoryTargetTokens <= 0 {
		return fmt.Errorf("OMP context history target must be positive")
	}
	if request.Policy.Fallback != config.OMPContextFallbackBlock && request.Policy.Fallback != config.OMPContextFallbackCanonicalFull {
		return fmt.Errorf("OMP context fallback policy is invalid")
	}
	return nil
}

func missingWorkflowContextCapability(request WorkflowContextRuntimeRequest) string {
	capabilities := request.Capabilities
	checks := []struct {
		name string
		ok   bool
	}{
		{"executable-identity", capabilities.ExecutableIdentity},
		{"version", strings.TrimSpace(capabilities.Version) != ""},
		{"settings-schema", capabilities.SettingsSchema},
		{"overlay-readback", capabilities.OverlayReadback},
		{"pre-compaction-event", capabilities.PreCompactionEvent},
		{"post-compaction-event", capabilities.PostCompactionEvent},
		{"canonical-injection", capabilities.CanonicalInjection},
		{"admission-blocking", capabilities.AdmissionBlocking},
		{"cleanup-readback", capabilities.CleanupReadback},
		{"probe-source", strings.TrimSpace(capabilities.ProbeSource) != ""},
		{"checked-at", !capabilities.CheckedAt.IsZero()},
	}
	for _, check := range checks {
		if !check.ok {
			return check.name
		}
	}
	if request.RootClass == config.OMPContextRuntimeNoSession && !capabilities.NoSession {
		return "no-session"
	}
	if request.RootClass == config.OMPContextRuntimeIsolatedTaskOwned && !capabilities.IsolatedTaskRoot {
		return "isolated-task-root"
	}
	if request.Policy.MemoryMode == config.OMPContextMemoryShadow && !capabilities.MemoryInterception {
		return "memory-interception"
	}
	return ""
}

func workflowContextHistoryCredits(
	binding promptlayer.OMPContextBindingReceipt,
	after map[string]int,
	target int,
) ([]WorkflowContextHistoryCredit, error) {
	rows := make([]WorkflowContextHistoryCredit, 0, len(binding.EligibleHistoryRefs))
	for _, ref := range binding.EligibleHistoryRefs {
		value, ok := after[ref.ID]
		if !ok || value <= 0 || value > target || value > ref.TokenEstimate {
			return nil, fmt.Errorf("unverified OMP history credit: %s", ref.ID)
		}
		rows = append(rows, WorkflowContextHistoryCredit{
			ID: ref.ID, SourceRef: ref.SourceRef, PriorHash: ref.BodyHash, Action: "compact_history",
			Reason: ref.Reason, TokenBefore: ref.TokenEstimate, TokenAfter: value,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}

func cleanupWorkflowContextRuntime(ctx context.Context, driver WorkflowContextProcessDriver, receipt *WorkflowContextRuntimeReceipt) error {
	receipt.Cleanup.Attempted = true
	if err := driver.Cleanup(ctx); err != nil {
		receipt.Cleanup.Reason = "cleanup-failed"
		return fmt.Errorf("cleanup OMP context runtime: %w", err)
	}
	after, err := driver.ArtifactCount(ctx)
	if err != nil {
		receipt.Cleanup.Reason = "cleanup-readback-failed"
		return fmt.Errorf("read OMP cleanup result: %w", err)
	}
	receipt.ArtifactCounts.AfterCleanup = after
	if after != 0 {
		receipt.Cleanup.Reason = "artifacts-remain"
		return fmt.Errorf("OMP context runtime cleanup left %d artifacts", after)
	}
	receipt.Cleanup.Verified = true
	receipt.Cleanup.Reason = "verified"
	return nil
}

func rollbackWorkflowContextOverlay(ctx context.Context, request WorkflowContextRuntimeRequest, receipt *WorkflowContextRuntimeReceipt) error {
	readback, err := ApplyWorkflowContextOverlay(ctx, request.Overlay, WorkflowContextOverlayRequest{
		HistoryMode: config.OMPContextHistoryShadow, MemoryMode: request.Policy.MemoryMode, Reason: receipt.Fallback.Reason,
	})
	if err != nil {
		return err
	}
	receipt.Mode = workflowContextModeReceipt(readback)
	return nil
}

func workflowContextModeReceipt(readback WorkflowContextOverlayReadback) WorkflowContextModeReceipt {
	return WorkflowContextModeReceipt{
		RequestedHistoryMode: readback.RequestedHistoryMode,
		EffectiveHistoryMode: readback.EffectiveHistoryMode,
		EffectiveMemoryMode:  readback.EffectiveMemoryMode,
		PreviousHistoryMode:  readback.PreviousHistoryMode,
		OverlayHash:          readback.OverlayHash,
		ReadbackHash:         readback.ReadbackHash,
	}
}

func workflowContextFailureReason(runErr, cleanupErr error) string {
	if cleanupErr != nil {
		return "runtime-cleanup-failed"
	}
	if errors.Is(runErr, promptlayer.ErrOMPContextBindingUnavailable) {
		return "ephemeral-state-unavailable"
	}
	if runErr != nil && strings.Contains(runErr.Error(), "unexpected OMP context runtime event") {
		return "event-sequence-invalid"
	}
	if runErr != nil && strings.Contains(runErr.Error(), "history credit") {
		return "history-credit-unverified"
	}
	return "rehydration-verification-failed"
}

func finishWorkflowContextRuntime(
	request WorkflowContextRuntimeRequest,
	receipt WorkflowContextRuntimeReceipt,
	runErr error,
) (WorkflowContextRuntimeReceipt, error) {
	if request.ReceiptWriter == nil {
		return receipt, runErr
	}
	writeErr := request.ReceiptWriter.Write(receipt)
	if writeErr != nil {
		return receipt, errors.Join(runErr, writeErr)
	}
	return receipt, runErr
}
