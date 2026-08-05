package cli

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextRuntimeManaged_MissingCanonicalSourceBlocksBeforeDriverAdmission(t *testing.T) {
	t.Parallel()
	request := newWorkflowContextRuntimeFixture(t)
	driver := &recordingManagedWorkflowContextDriver{}
	request.Driver = driver
	request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
	request.CanonicalSource = nil

	receipt, err := NewWorkflowContextRuntimeSupervisor(nil).RunManaged(context.Background(), request)

	require.Error(t, err)
	assert.Equal(t, WorkflowContextOutcomeBlocked, receipt.Outcome)
	assert.Zero(t, driver.bindCalls)
	assert.Zero(t, driver.runCalls)
	assert.Zero(t, driver.dispatchCalls)
}

func TestWorkflowContextRuntimeManaged_NonManagedDriverBlocksBeforeDriverAdmission(t *testing.T) {
	t.Parallel()
	request := newWorkflowContextRuntimeFixture(t)
	driver := &fakeWorkflowContextDriver{}
	request.Driver = driver
	request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
	request.CanonicalSource = workflowContextCanonicalSourceFunc(func(_ context.Context, opts promptlayer.ContextDeliveryOptions) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
		delivery, err := promptlayer.BuildContextDelivery(opts)
		return delivery, request.Binding.Ephemeral, err
	})

	receipt, err := NewWorkflowContextRuntimeSupervisor(nil).RunManaged(context.Background(), request)

	require.Error(t, err)
	assert.Equal(t, WorkflowContextOutcomeBlocked, receipt.Outcome)
	assert.NotContains(t, receipt.PhaseSequence, "admitted")
	assert.Zero(t, driver.runCalls)
	assert.Zero(t, driver.cleanupCalls)
}

func TestWorkflowContextRuntimeManaged_BindsHashesAndDispatchesPostRebuildOnce(t *testing.T) {
	t.Parallel()
	request := newWorkflowContextRuntimeFixture(t)
	expected, err := promptlayer.BuildOMPContextBinding(request.Binding)
	require.NoError(t, err)
	request.Binding.Delivery.Prompt = "stale prompt must never reach managed dispatch"
	driver := &recordingManagedWorkflowContextDriver{events: []WorkflowContextRuntimeEvent{
		{Kind: WorkflowContextEventPreCompaction},
		{Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 2}},
		{Kind: WorkflowContextEventPostCompaction},
	}, artifacts: 1}
	request.Driver = driver
	request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
	rebuildCalls := 0
	rebuildWhileRunActive := false
	canonicalPrompt := ""
	request.CanonicalSource = workflowContextCanonicalSourceFunc(func(_ context.Context, opts promptlayer.ContextDeliveryOptions) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
		rebuildCalls++
		rebuildWhileRunActive = driver.isRunActive()
		delivery, rebuildErr := promptlayer.BuildContextDelivery(opts)
		canonicalPrompt = delivery.Prompt
		return delivery, request.Binding.Ephemeral, rebuildErr
	})

	receipt, err := NewWorkflowContextRuntimeSupervisor(nil).RunManaged(context.Background(), request)

	require.NoError(t, err)
	assert.Equal(t, WorkflowContextOutcomeAdmitted, receipt.Outcome)
	assert.Equal(t, 1, rebuildCalls, "post-compaction admission must rebuild from the authoritative source")
	require.Equal(t, 1, driver.bindCalls)
	assert.Equal(t, expected.BindingHash, driver.binding.BindingHash)
	assert.Equal(t, expected.OptionsHash, driver.binding.OptionsHash)
	assert.Equal(t, runtimeHash(request.Binding.SessionID), driver.binding.SessionHash)
	assert.NotEmpty(t, driver.binding.NonceHash)
	assertRuntimeSHA256(t, driver.binding.NonceHash)
	assert.Equal(t, 1, driver.runCalls)
	assert.Equal(t, 1, driver.dispatchCalls)
	assert.True(t, rebuildWhileRunActive, "authoritative rebuild must happen while the bound process is alive")
	assert.True(t, driver.dispatchWhileRunActive, "provider dispatch must happen while the bound process is alive")
	assert.Equal(t, WorkflowContextDispatchOptimized, driver.dispatchMode)
	assert.Equal(t, canonicalPrompt, driver.dispatchedPrompt)
	assert.NotEqual(t, request.Binding.Delivery.Prompt, driver.dispatchedPrompt)
	assert.Equal(t, request.Binding.Ephemeral.OriginalTask, driver.dispatchedOriginalTask)
	assert.Equal(t, []string{"checkpointed", "compacted", "rehydrated", "admitted"}, receipt.PhaseSequence)
	assert.True(t, receipt.ExactMatch)
	assert.True(t, receipt.Cleanup.Verified)
	assert.Equal(t, 1, driver.cleanupCalls)
}

type recordingManagedWorkflowContextDriver struct {
	mu                     sync.Mutex
	events                 []WorkflowContextRuntimeEvent
	before                 func(WorkflowContextRuntimeEvent)
	binding                WorkflowContextBridgeBinding
	ackFactory             func(WorkflowContextBridgeBinding, WorkflowContextDispatch) WorkflowContextDispatchAck
	dispatchErr            error
	artifacts              int
	bindCalls              int
	runCalls               int
	dispatchCalls          int
	cleanupCalls           int
	runActive              bool
	dispatchWhileRunActive bool
	dispatchMode           string
	dispatchedPrompt       string
	dispatchedOriginalTask string
	observation            *WorkflowContextManagedRPCObservation
}

var _ WorkflowContextManagedProcessDriver = (*recordingManagedWorkflowContextDriver)(nil)

func (driver *recordingManagedWorkflowContextDriver) Cleanup(context.Context) error {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	driver.cleanupCalls++
	driver.artifacts = 0
	return nil
}

func (driver *recordingManagedWorkflowContextDriver) Bind(_ context.Context, binding WorkflowContextBridgeBinding) error {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	driver.bindCalls++
	driver.binding = binding
	return nil
}

func (driver *recordingManagedWorkflowContextDriver) Run(_ context.Context, emit func(WorkflowContextRuntimeEvent) error) error {
	driver.mu.Lock()
	driver.runCalls++
	driver.runActive = true
	driver.mu.Unlock()
	defer func() {
		driver.mu.Lock()
		driver.runActive = false
		driver.mu.Unlock()
	}()
	for _, event := range driver.events {
		if driver.before != nil {
			driver.before(event)
		}
		if err := emit(event); err != nil {
			return err
		}
	}
	return nil
}

func (driver *recordingManagedWorkflowContextDriver) Dispatch(_ context.Context, dispatch WorkflowContextDispatch) (WorkflowContextDispatchAck, error) {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	driver.dispatchCalls++
	driver.dispatchWhileRunActive = driver.runActive
	driver.dispatchMode = dispatch.Mode
	driver.dispatchedPrompt = dispatch.Delivery.Prompt
	driver.dispatchedOriginalTask = dispatch.Transient.OriginalTask()
	if driver.dispatchErr != nil {
		return WorkflowContextDispatchAck{}, driver.dispatchErr
	}
	if driver.ackFactory != nil {
		return driver.ackFactory(driver.binding, dispatch), nil
	}
	return validWorkflowContextDispatchAck(driver.binding), nil
}

func (driver *recordingManagedWorkflowContextDriver) ArtifactCount(context.Context) (int, error) {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	return driver.artifacts, nil
}
func (driver *recordingManagedWorkflowContextDriver) Observation() WorkflowContextManagedRPCObservation {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.observation != nil {
		return *driver.observation
	}
	cycles := driver.dispatchCalls
	return WorkflowContextManagedRPCObservation{
		ProviderTurns: 2 + cycles, PreACKs: cycles, PostACKs: cycles,
		NativeStarts: cycles, NativeEnds: cycles,
		CanonicalReadmissions: cycles, EphemeralReadmissions: cycles,
		SameProcess: true, SameSession: true, Sandboxed: true, ProviderObserved: cycles > 0,
		ProviderAuthorityDigest: workflowContextRuntimeHash("recording-loopback-authority"),
	}
}

func (driver *recordingManagedWorkflowContextDriver) isRunActive() bool {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	return driver.runActive
}

func validWorkflowContextDispatchAck(binding WorkflowContextBridgeBinding) WorkflowContextDispatchAck {
	return WorkflowContextDispatchAck{
		SchemaVersion:    "autopus.omp-context-dispatch-ack.v1",
		BindingHash:      binding.BindingHash,
		OptionsHash:      binding.OptionsHash,
		SessionHash:      binding.SessionHash,
		NonceHash:        binding.NonceHash,
		ProviderObserved: true,
	}
}

func workflowContextDispatchError() error {
	return errors.New("provider dispatch rejected before observation")
}
