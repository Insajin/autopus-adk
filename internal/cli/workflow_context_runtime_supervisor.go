package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

type WorkflowContextRuntimeSupervisor struct {
	store *promptlayer.OMPContextTransientStore
}

func NewWorkflowContextRuntimeSupervisor(store *promptlayer.OMPContextTransientStore) *WorkflowContextRuntimeSupervisor {
	if store == nil {
		store = promptlayer.NewOMPContextTransientStore()
	}
	return &WorkflowContextRuntimeSupervisor{store: store}
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-003: public admission boundary for checkpoint, compaction, rehydration, and dispatch.
// @AX:REASON [AUTO]: installed-canary and workflow callers depend on the ordered receipt and cleanup contract exposed here.
// @AX:WARN [AUTO]: the runtime admission state machine has cyclomatic complexity 28.
// @AX:REASON [AUTO]: gocyclo reports 28 across validation, checkpoint, compaction, rehydration, dispatch, and cleanup outcomes.
// @AX:NOTE [AUTO]: exported Run spans more than 100 lines without a dedicated Go doc contract.
func (supervisor *WorkflowContextRuntimeSupervisor) Run(
	ctx context.Context,
	request WorkflowContextRuntimeRequest,
	dispatch WorkflowContextDispatchFunc,
) (WorkflowContextRuntimeReceipt, error) {
	if err := validateWorkflowContextRuntimeMetadata(request); err != nil {
		cleanupErr := cleanupUntrustedWorkflowContextRuntime(request)
		return WorkflowContextRuntimeReceipt{}, errors.Join(err, cleanupErr)
	}
	receipt := newWorkflowContextRuntimeReceipt(request)
	if err := validateWorkflowContextRuntimeRequest(request); err != nil {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Fallback.Reason = "runtime-request-invalid"
		return supervisor.failWorkflowContextRuntime(request, receipt, "", err)
	}
	preview, err := promptlayer.BuildOMPContextBinding(request.Binding)
	if err != nil {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Fallback.Reason = "runtime-binding-invalid"
		return supervisor.failWorkflowContextRuntime(request, receipt, "", err)
	}
	if err := verifyWorkflowContextPromotion(request, preview, time.Now()); err != nil {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Mode.EffectiveHistoryMode = config.OMPContextHistoryShadow
		receipt.Fallback.Reason = workflowContextPromotionFailureReason(err)
		return supervisor.failWorkflowContextRuntime(request, receipt, "", err)
	}
	populateWorkflowContextPromotionReceipt(&receipt, request.Promotion)
	if missing := missingWorkflowContextCapability(request); missing != "" {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Mode.EffectiveHistoryMode = config.OMPContextHistoryShadow
		receipt.Fallback.Reason = "capability-missing:" + missing
		return supervisor.failWorkflowContextRuntime(request, receipt, "", fmt.Errorf("OMP context capability missing: %s", missing))
	}
	if request.Driver == nil || request.Overlay == nil {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Fallback.Reason = "runtime-dependency-unavailable"
		return supervisor.failWorkflowContextRuntime(request, receipt, "", fmt.Errorf("OMP context driver and overlay are required"))
	}
	readback, err := ApplyWorkflowContextOverlay(ctx, request.Overlay, WorkflowContextOverlayRequest{
		HistoryMode: request.Policy.HistoryMode, MemoryMode: request.Policy.MemoryMode, Reason: "active-admission",
	})
	if err != nil {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Fallback.Reason = "overlay-readback-mismatch"
		return supervisor.failWorkflowContextRuntime(request, receipt, "", err)
	}
	receipt.Mode = workflowContextModeReceipt(readback)

	before, err := request.Driver.ArtifactCount(ctx)
	if err != nil {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Fallback.Reason = "artifact-count-unavailable"
		return supervisor.failWorkflowContextRuntime(request, receipt, "", fmt.Errorf("inspect OMP context artifacts: %w", err))
	}
	if before < 0 {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Fallback.Reason = "artifact-count-unavailable"
		return supervisor.failWorkflowContextRuntime(request, receipt, "", fmt.Errorf("OMP context artifact count is negative"))
	}
	receipt.ArtifactCounts.Before = before

	var binding promptlayer.OMPContextBindingReceipt
	var terminal promptlayer.OMPContextTerminalReceipt
	stage := 0
	runErr := request.Driver.Run(ctx, func(event WorkflowContextRuntimeEvent) error {
		switch {
		case stage == 0 && event.Kind == WorkflowContextEventPreCompaction:
			checkpoint, checkpointErr := supervisor.store.Checkpoint(request.Binding)
			if checkpointErr != nil {
				return checkpointErr
			}
			binding = checkpoint
			populateWorkflowContextBindingReceipt(&receipt, request, binding)
			receipt.PhaseSequence = append(receipt.PhaseSequence, "checkpointed")
			stage = 1
			return nil
		case stage == 1 && event.Kind == WorkflowContextEventCompacted:
			credits, creditErr := workflowContextHistoryCredits(binding, event.HistoryAfterTokens, request.Policy.HistoryTargetTokens)
			if creditErr != nil {
				return creditErr
			}
			receipt.HistoryCreditRows = credits
			receipt.PhaseSequence = append(receipt.PhaseSequence, "compacted")
			stage = 2
			return nil
		case stage == 2 && event.Kind == WorkflowContextEventPostCompaction:
			var rehydrateErr error
			terminal, rehydrateErr = supervisor.store.Rehydrate(binding.BindingHash, request.Binding.DeliveryOptions,
				func(view promptlayer.OMPContextTransientView) error {
					receipt.PhaseSequence = append(receipt.PhaseSequence, "rehydrated")
					if dispatch == nil {
						return fmt.Errorf("OMP context provider dispatch callback is required")
					}
					return dispatch(ctx, WorkflowContextDispatch{
						Mode: WorkflowContextDispatchOptimized, Delivery: request.Binding.Delivery, Transient: view,
					})
				})
			if rehydrateErr != nil {
				return rehydrateErr
			}
			receipt.ExactMatch = terminal.ExactMatch
			receipt.Outcome = WorkflowContextOutcomeAdmitted
			receipt.PhaseSequence = append(receipt.PhaseSequence, "admitted")
			stage = 3
			return nil
		default:
			return fmt.Errorf("unexpected OMP context runtime event %q at stage %d", event.Kind, stage)
		}
	})

	maintenance, cancelMaintenance := workflowContextMaintenanceContext()
	cleanupErr := cleanupWorkflowContextRuntime(maintenance, request.Driver, &receipt)
	cancelMaintenance()
	if runErr == nil && stage != 3 {
		runErr = fmt.Errorf("OMP context runtime event stream ended at stage %d", stage)
	}
	if runErr == nil && cleanupErr == nil {
		finished, finishErr := finishWorkflowContextRuntime(request, receipt, nil)
		if finishErr == nil {
			return finished, nil
		}
		finished.Outcome = WorkflowContextOutcomeBlocked
		finished.Fallback.Mode = config.OMPContextFallbackBlock
		finished.Fallback.Reason = "receipt-write-failed"
		request.ReceiptWriter = nil
		return supervisor.failWorkflowContextRuntime(request, finished, "", finishErr)
	}
	if terminal.Reason == "required-context-mismatch" && cleanupErr == nil {
		return supervisor.runCanonicalFallback(ctx, request, receipt, dispatch)
	}
	receipt.Outcome = WorkflowContextOutcomeBlocked
	receipt.Fallback.Mode = config.OMPContextFallbackBlock
	receipt.Fallback.Reason = workflowContextFailureReason(runErr, cleanupErr)
	return supervisor.failWorkflowContextRuntime(request, receipt, binding.BindingHash, errors.Join(runErr, cleanupErr))
}

func (supervisor *WorkflowContextRuntimeSupervisor) runCanonicalFallback(
	ctx context.Context,
	request WorkflowContextRuntimeRequest,
	receipt WorkflowContextRuntimeReceipt,
	dispatch WorkflowContextDispatchFunc,
) (WorkflowContextRuntimeReceipt, error) {
	if request.Policy.Fallback != config.OMPContextFallbackCanonicalFull || request.CanonicalSource == nil {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Fallback.Reason = "required-source-changed"
		return finishWorkflowContextRuntime(request, receipt, fmt.Errorf("independent canonical fallback is unavailable"))
	}
	receipt.Fallback.Reason = "required-source-changed"
	maintenance, cancelMaintenance := workflowContextMaintenanceContext()
	err := rollbackWorkflowContextOverlay(maintenance, request, &receipt)
	cancelMaintenance()
	if err != nil {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Fallback.Reason = "rollback-readback-mismatch"
		return finishWorkflowContextRuntime(request, receipt, err)
	}
	delivery, ephemeral, err := request.CanonicalSource.Rebuild(ctx, request.Binding.DeliveryOptions)
	if err != nil {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Fallback.Reason = "canonical-full-rebuild-failed"
		return finishWorkflowContextRuntime(request, receipt, err)
	}
	if err := promptlayer.VerifyContextDeliveryForOptions(request.Binding.DeliveryOptions, delivery); err != nil {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Fallback.Reason = "canonical-full-verification-failed"
		return finishWorkflowContextRuntime(request, receipt, err)
	}
	fallbackInput := request.Binding
	fallbackInput.Delivery = delivery
	fallbackInput.Ephemeral = ephemeral
	fallbackInput.History = nil
	fallbackInput.ShadowPlan = nil
	fallbackBinding, err := promptlayer.BuildOMPContextBinding(fallbackInput)
	if err != nil {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Fallback.Reason = "canonical-full-binding-invalid"
		return finishWorkflowContextRuntime(request, receipt, err)
	}
	if dispatch == nil {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Fallback.Reason = "canonical-full-dispatch-unavailable"
		return finishWorkflowContextRuntime(request, receipt, fmt.Errorf("OMP context provider dispatch callback is required"))
	}
	if err := dispatch(ctx, WorkflowContextDispatch{
		Mode: WorkflowContextDispatchCanonicalFull, Delivery: delivery, Ephemeral: ephemeral,
	}); err != nil {
		receipt.Outcome = WorkflowContextOutcomeBlocked
		receipt.Fallback.Mode = config.OMPContextFallbackBlock
		receipt.Fallback.Reason = "canonical-full-dispatch-failed"
		return finishWorkflowContextRuntime(request, receipt, err)
	}
	receipt.Outcome = WorkflowContextOutcomeFallback
	receipt.ExactMatch = false
	receipt.PhaseSequence = append(receipt.PhaseSequence, "fallback_full", "admitted")
	receipt.Fallback = WorkflowContextFallbackReceipt{
		Mode: config.OMPContextFallbackCanonicalFull, Reason: "required-source-changed", Integrity: "verified",
		SnapshotHash: delivery.SnapshotHash, PromptManifestHash: delivery.PromptManifestHash,
		FullDocumentRefs: fallbackBinding.FullDocumentRefs,
	}
	return finishWorkflowContextRuntime(request, receipt, nil)
}

func newWorkflowContextRuntimeReceipt(request WorkflowContextRuntimeRequest) WorkflowContextRuntimeReceipt {
	return WorkflowContextRuntimeReceipt{
		SchemaVersion: WorkflowContextRuntimeReceiptSchemaVersion, Event: "terminal",
		WorkspaceID: request.Binding.WorkspaceID, SpecID: request.Binding.SpecID, TaskID: request.Binding.TaskID,
		Phase: request.Binding.Phase, SessionID: request.Binding.SessionID, Capabilities: request.Capabilities,
		RootClass: request.RootClass, FullDocumentRefs: []promptlayer.OMPContextDocumentReference{},
		RequiredEphemeralRefs: []promptlayer.OMPContextHashedReference{}, FrozenFindingIDs: []string{},
		WorkerResultFields: promptlayer.OMPWorkerResultSchema(), HistoryCreditRows: []WorkflowContextHistoryCredit{},
		ShadowCandidateRefs: []promptlayer.OMPContextPlanReference{}, DocumentOmissions: []string{},
		MemoryInjections: []string{}, PhaseSequence: []string{},
		Mode:     WorkflowContextModeReceipt{RequestedHistoryMode: request.Policy.HistoryMode, EffectiveMemoryMode: request.Policy.MemoryMode},
		Fallback: WorkflowContextFallbackReceipt{Mode: WorkflowContextFallbackNone},
	}
}

func populateWorkflowContextBindingReceipt(receipt *WorkflowContextRuntimeReceipt, request WorkflowContextRuntimeRequest, binding promptlayer.OMPContextBindingReceipt) {
	receipt.BindingHash = binding.BindingHash
	receipt.OptionsHash = binding.OptionsHash
	receipt.SnapshotHash = binding.SnapshotHash
	receipt.PromptManifestHash = binding.PromptManifestHash
	receipt.FullDocumentRefs = append([]promptlayer.OMPContextDocumentReference(nil), binding.FullDocumentRefs...)
	receipt.RequiredEphemeralRefs = append([]promptlayer.OMPContextHashedReference(nil), binding.RequiredEphemeralRefs...)
	receipt.FrozenFindingIDs = append([]string(nil), request.Binding.Ephemeral.FrozenFindingIDs...)
	receipt.ShadowCandidateRefs = append([]promptlayer.OMPContextPlanReference(nil), binding.ShadowPlanRefs...)
}

func workflowContextHistoryCredits(binding promptlayer.OMPContextBindingReceipt, after map[string]int, target int) ([]WorkflowContextHistoryCredit, error) {
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
