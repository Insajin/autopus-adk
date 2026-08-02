package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextRuntime_ExactRehydrateAdmitsOptimizedDispatchOnce(t *testing.T) {
	t.Parallel()
	request := newWorkflowContextRuntimeFixture(t)
	shadowDocument := request.Binding.Delivery.RequiredDocuments[len(request.Binding.Delivery.RequiredDocuments)-1]
	request.Binding.ShadowPlan = &promptlayer.OMPContextShadowPlan{
		SchemaVersion: "autopus.context_plan.v2", ShadowOnly: true, ActiveMode: "full", CandidateMode: "jit",
		SelectedReferences: []promptlayer.OMPContextPlanReference{{
			SourceRef: shadowDocument.SourceRef, SourceHash: shadowDocument.SourceHash,
		}},
	}
	attachValidWorkflowContextPromotion(t, &request)
	driver := &fakeWorkflowContextDriver{events: []WorkflowContextRuntimeEvent{
		{Kind: WorkflowContextEventPreCompaction},
		{Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 2}},
		{Kind: WorkflowContextEventPostCompaction},
	}, artifacts: 3}
	request.Driver = driver
	request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
	dispatcher := &recordingWorkflowContextDispatcher{}

	receipt, err := NewWorkflowContextRuntimeSupervisor(nil).Run(context.Background(), request, dispatcher.Dispatch)
	require.NoError(t, err)
	assert.Equal(t, []string{"checkpointed", "compacted", "rehydrated", "admitted"}, receipt.PhaseSequence)
	assert.Equal(t, WorkflowContextOutcomeAdmitted, receipt.Outcome)
	assert.True(t, receipt.ExactMatch)
	assert.Equal(t, 1, dispatcher.optimized)
	assert.Zero(t, dispatcher.canonical)
	assert.Equal(t, "implement runtime supervisor", dispatcher.originalTask)
	assert.Equal(t, request.Binding.Delivery.SnapshotHash, receipt.SnapshotHash)
	assert.Equal(t, request.Binding.Delivery.PromptManifestHash, receipt.PromptManifestHash)
	require.Len(t, receipt.HistoryCreditRows, 1)
	assert.Equal(t, 2, receipt.HistoryCreditRows[0].TokenAfter)
	require.Len(t, receipt.ShadowCandidateRefs, 1)
	assert.Equal(t, shadowDocument.SourceRef, receipt.ShadowCandidateRefs[0].SourceRef)
	assert.Len(t, receipt.FullDocumentRefs, len(request.Binding.Delivery.RequiredDocuments))
	assert.Empty(t, receipt.DocumentOmissions)
	assert.Empty(t, receipt.MemoryInjections)
	assert.Equal(t, WorkflowContextArtifactCounts{Before: 3, AfterCleanup: 0}, receipt.ArtifactCounts)
	assert.True(t, receipt.Cleanup.Verified)
	assert.Equal(t, 1, driver.cleanupCalls)
}

func TestWorkflowContextRuntime_SourceChangeUsesIndependentCanonicalFallbackOnly(t *testing.T) {
	t.Parallel()
	request := newWorkflowContextRuntimeFixture(t)
	acceptance := filepath.Join(request.Binding.DeliveryOptions.Root, filepath.FromSlash(runtimeSpecDir), "acceptance.md")
	driver := &fakeWorkflowContextDriver{events: []WorkflowContextRuntimeEvent{
		{Kind: WorkflowContextEventPreCompaction},
		{Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 2}},
		{Kind: WorkflowContextEventPostCompaction},
	}, artifacts: 1, before: func(event WorkflowContextRuntimeEvent) {
		if event.Kind == WorkflowContextEventPostCompaction {
			require.NoError(t, os.WriteFile(acceptance, []byte("changed canonical acceptance"), 0o600))
		}
	}}
	request.Driver = driver
	request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
	request.CanonicalSource = workflowContextCanonicalSourceFunc(func(_ context.Context, opts promptlayer.ContextDeliveryOptions) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
		delivery, err := promptlayer.BuildContextDelivery(opts)
		return delivery, request.Binding.Ephemeral, err
	})
	dispatcher := &recordingWorkflowContextDispatcher{}

	receipt, err := NewWorkflowContextRuntimeSupervisor(nil).Run(context.Background(), request, dispatcher.Dispatch)
	require.NoError(t, err)
	assert.Equal(t, WorkflowContextOutcomeFallback, receipt.Outcome)
	assert.Equal(t, "required-source-changed", receipt.Fallback.Reason)
	assert.Equal(t, "verified", receipt.Fallback.Integrity)
	assert.Zero(t, dispatcher.optimized)
	assert.Equal(t, 1, dispatcher.canonical)
	assert.Equal(t, []string{"checkpointed", "compacted", "fallback_full", "admitted"}, receipt.PhaseSequence)
	assert.Equal(t, config.OMPContextHistoryShadow, receipt.Mode.EffectiveHistoryMode)
}

func TestWorkflowContextRuntime_MissingTransientAndCapabilityFailClosed(t *testing.T) {
	t.Parallel()
	t.Run("transient removed", func(t *testing.T) {
		request := newWorkflowContextRuntimeFixture(t)
		store := promptlayer.NewOMPContextTransientStore()
		binding, err := promptlayer.BuildOMPContextBinding(request.Binding)
		require.NoError(t, err)
		request.Driver = &fakeWorkflowContextDriver{events: []WorkflowContextRuntimeEvent{
			{Kind: WorkflowContextEventPreCompaction}, {Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 2}}, {Kind: WorkflowContextEventPostCompaction},
		}, artifacts: 1, after: func(event WorkflowContextRuntimeEvent) {
			if event.Kind == WorkflowContextEventPreCompaction {
				_, abortErr := store.Abort(binding.BindingHash, "test-transient-loss")
				require.NoError(t, abortErr)
			}
		}}
		request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
		dispatcher := &recordingWorkflowContextDispatcher{}
		receipt, runErr := NewWorkflowContextRuntimeSupervisor(store).Run(context.Background(), request, dispatcher.Dispatch)
		require.Error(t, runErr)
		assert.Equal(t, "ephemeral-state-unavailable", receipt.Fallback.Reason)
		assert.Zero(t, dispatcher.calls())
	})

	t.Run("capability missing", func(t *testing.T) {
		request := newWorkflowContextRuntimeFixture(t)
		driver := &fakeWorkflowContextDriver{}
		request.Driver = driver
		request.Capabilities.CanonicalInjection = false
		dispatcher := &recordingWorkflowContextDispatcher{}
		receipt, err := NewWorkflowContextRuntimeSupervisor(nil).Run(context.Background(), request, dispatcher.Dispatch)
		require.Error(t, err)
		assert.Equal(t, "capability-missing:canonical-injection", receipt.Fallback.Reason)
		assert.Zero(t, dispatcher.calls())
		assert.Zero(t, driver.runCalls)
	})
}

func TestWorkflowContextOverlay_PromotionAndRollbackRequireMatchingReadbackHashes(t *testing.T) {
	t.Parallel()
	overlay := newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
	promoted, err := ApplyWorkflowContextOverlay(context.Background(), overlay, WorkflowContextOverlayRequest{
		HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff, Reason: "promotion-gates-pass",
	})
	require.NoError(t, err)
	assert.Equal(t, promoted.OverlayHash, promoted.ReadbackHash)
	rolledBack, err := ApplyWorkflowContextOverlay(context.Background(), overlay, WorkflowContextOverlayRequest{
		HistoryMode: config.OMPContextHistoryShadow, MemoryMode: config.OMPContextMemoryOff, Reason: "quality-regression",
	})
	require.NoError(t, err)
	assert.NotEqual(t, promoted.ReadbackHash, rolledBack.ReadbackHash)
	assert.Equal(t, config.OMPContextHistoryActive, rolledBack.PreviousHistoryMode)

	mismatch := activeOverlayReadback()
	mismatch.ReadbackHash = runtimeHash("different-readback")
	_, err = ApplyWorkflowContextOverlay(context.Background(), newFakeWorkflowContextOverlay(t, mismatch), WorkflowContextOverlayRequest{
		HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff,
	})
	require.ErrorContains(t, err, "readback hash mismatch")
}

type fakeWorkflowContextDriver struct {
	events       []WorkflowContextRuntimeEvent
	artifacts    int
	runCalls     int
	cleanupCalls int
	before       func(WorkflowContextRuntimeEvent)
	after        func(WorkflowContextRuntimeEvent)
}

func (d *fakeWorkflowContextDriver) ArtifactCount(context.Context) (int, error) {
	return d.artifacts, nil
}
func (d *fakeWorkflowContextDriver) Cleanup(context.Context) error {
	d.cleanupCalls++
	d.artifacts = 0
	return nil
}
func (d *fakeWorkflowContextDriver) Run(_ context.Context, emit func(WorkflowContextRuntimeEvent) error) error {
	d.runCalls++
	for _, event := range d.events {
		if d.before != nil {
			d.before(event)
		}
		if err := emit(event); err != nil {
			return err
		}
		if d.after != nil {
			d.after(event)
		}
	}
	return nil
}

type fakeWorkflowContextOverlay struct {
	t         *testing.T
	mu        sync.Mutex
	readbacks []WorkflowContextOverlayReadback
}

func newFakeWorkflowContextOverlay(t *testing.T, readbacks ...WorkflowContextOverlayReadback) *fakeWorkflowContextOverlay {
	return &fakeWorkflowContextOverlay{t: t, readbacks: readbacks}
}
func (o *fakeWorkflowContextOverlay) Apply(_ context.Context, _ WorkflowContextOverlayRequest) (WorkflowContextOverlayReadback, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.readbacks) == 0 {
		return WorkflowContextOverlayReadback{}, errors.New("unexpected overlay apply")
	}
	value := o.readbacks[0]
	o.readbacks = o.readbacks[1:]
	return value, nil
}

type recordingWorkflowContextDispatcher struct {
	mu           sync.Mutex
	optimized    int
	canonical    int
	originalTask string
}

func (d *recordingWorkflowContextDispatcher) Dispatch(_ context.Context, input WorkflowContextDispatch) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if input.Mode == WorkflowContextDispatchOptimized {
		d.optimized++
		d.originalTask = input.Transient.OriginalTask()
	} else if input.Mode == WorkflowContextDispatchCanonicalFull {
		d.canonical++
		d.originalTask = input.Ephemeral.OriginalTask
	}
	return nil
}
func (d *recordingWorkflowContextDispatcher) calls() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.optimized + d.canonical
}
