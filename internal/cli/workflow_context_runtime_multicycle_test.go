package cli

import (
	"context"
	"testing"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextRuntimeManaged_TwoCompactionsRepeatAdmissionBeforeSingleRollbackCleanup(t *testing.T) {
	request := newWorkflowContextRuntimeFixture(t)
	driver := &recordingManagedWorkflowContextDriver{events: []WorkflowContextRuntimeEvent{
		{Kind: WorkflowContextEventPreCompaction},
		{Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 2}},
		{Kind: WorkflowContextEventPostCompaction},
		{Kind: WorkflowContextEventPreCompaction},
		{Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 1}},
		{Kind: WorkflowContextEventPostCompaction},
	}, artifacts: 1}
	overlay := &countingWorkflowContextMultiCycleOverlay{
		readbacks: []WorkflowContextOverlayReadback{activeOverlayReadback(), shadowOverlayReadback()},
	}
	request.Driver = driver
	request.Overlay = overlay
	rebuildCalls := 0
	request.CanonicalSource = workflowContextCanonicalSourceFunc(func(
		_ context.Context, options promptlayer.ContextDeliveryOptions,
	) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
		rebuildCalls++
		delivery, err := promptlayer.BuildContextDelivery(options)
		return delivery, request.Binding.Ephemeral, err
	})

	receipt, err := NewWorkflowContextRuntimeSupervisor(nil).RunManaged(context.Background(), request)

	assert.NoError(t, err)
	assert.Equal(t, WorkflowContextOutcomeAdmitted, receipt.Outcome)
	assert.True(t, receipt.ExactMatch)
	assert.Equal(t, []string{
		"checkpointed", "compacted", "rehydrated",
		"checkpointed", "compacted", "rehydrated",
		"admitted",
	}, receipt.PhaseSequence)
	assert.Equal(t, 2, rebuildCalls, "each native compaction must rebuild canonical authority")
	assert.Equal(t, 2, driver.dispatchCalls, "each rebuild must cross an observed provider boundary")
	assert.Equal(t, 1, driver.bindCalls, "both cycles must reuse one bound managed session")
	assert.Equal(t, 1, driver.runCalls, "both cycles must run in one managed process")
	assert.Equal(t, 2, overlay.calls, "active overlay and final rollback must each apply once")
	assert.Equal(t, 1, driver.cleanupCalls, "the reusable process must be cleaned only after both cycles")
}

func TestWorkflowContextRuntimeManaged_RejectsZeroOrIncompleteCompactionCycles(t *testing.T) {
	firstCycle := []WorkflowContextRuntimeEvent{
		{Kind: WorkflowContextEventPreCompaction},
		{Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 2}},
		{Kind: WorkflowContextEventPostCompaction},
	}
	for _, test := range []struct {
		name          string
		events        []WorkflowContextRuntimeEvent
		wantStage     string
		wantDispatch  int
		wantRehydrate int
	}{
		{name: "zero cycles", wantStage: "stage 0"},
		{name: "second cycle stops after pre hook", events: append(append([]WorkflowContextRuntimeEvent{}, firstCycle...),
			WorkflowContextRuntimeEvent{Kind: WorkflowContextEventPreCompaction}), wantStage: "stage 1", wantDispatch: 1, wantRehydrate: 1},
		{name: "second cycle stops after compacted event", events: append(append([]WorkflowContextRuntimeEvent{}, firstCycle...),
			WorkflowContextRuntimeEvent{Kind: WorkflowContextEventPreCompaction},
			WorkflowContextRuntimeEvent{Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 1}}),
			wantStage: "stage 2", wantDispatch: 1, wantRehydrate: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := newWorkflowContextRuntimeFixture(t)
			driver := &recordingManagedWorkflowContextDriver{events: test.events, artifacts: 1}
			request.Driver = driver
			request.Overlay = &countingWorkflowContextMultiCycleOverlay{
				readbacks: []WorkflowContextOverlayReadback{activeOverlayReadback(), shadowOverlayReadback()},
			}
			rebuildCalls := 0
			request.CanonicalSource = workflowContextCanonicalSourceFunc(func(
				_ context.Context, options promptlayer.ContextDeliveryOptions,
			) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
				rebuildCalls++
				delivery, err := promptlayer.BuildContextDelivery(options)
				return delivery, request.Binding.Ephemeral, err
			})

			receipt, err := NewWorkflowContextRuntimeSupervisor(nil).RunManaged(context.Background(), request)

			require.ErrorContains(t, err, test.wantStage)
			assert.Equal(t, WorkflowContextOutcomeBlocked, receipt.Outcome)
			assert.NotContains(t, receipt.PhaseSequence, "admitted")
			assert.Equal(t, test.wantDispatch, driver.dispatchCalls)
			assert.Equal(t, test.wantRehydrate, rebuildCalls)
			assert.Equal(t, 1, driver.bindCalls)
			assert.Equal(t, 1, driver.runCalls)
			assert.Equal(t, 1, driver.cleanupCalls)
		})
	}
}

type countingWorkflowContextMultiCycleOverlay struct {
	readbacks []WorkflowContextOverlayReadback
	calls     int
}

func (overlay *countingWorkflowContextMultiCycleOverlay) Apply(
	_ context.Context, _ WorkflowContextOverlayRequest,
) (WorkflowContextOverlayReadback, error) {
	overlay.calls++
	if len(overlay.readbacks) == 0 {
		return WorkflowContextOverlayReadback{}, assert.AnError
	}
	readback := overlay.readbacks[0]
	overlay.readbacks = overlay.readbacks[1:]
	return readback, nil
}
