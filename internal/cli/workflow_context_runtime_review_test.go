package cli

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextRuntime_MissingPromotionAttestationFailsClosed(t *testing.T) {
	t.Parallel()
	request := newWorkflowContextRuntimeFixture(t)
	request.Promotion = promptlayer.OMPContextPromotionEvidenceV1{}
	request.Driver = &maintenanceContextDriver{}
	request.Overlay = &maintenanceContextOverlay{}

	receipt, err := NewWorkflowContextRuntimeSupervisor(nil).Run(context.Background(), request, nil)

	require.Error(t, err)
	assert.Equal(t, WorkflowContextOutcomeBlocked, receipt.Outcome)
	assert.Equal(t, config.OMPContextHistoryShadow, receipt.Mode.EffectiveHistoryMode)
	assert.Equal(t, "promotion-attestation-absent", receipt.Fallback.Reason)
	assert.Zero(t, request.Driver.(*maintenanceContextDriver).runCalls)
	assert.Equal(t, 1, request.Driver.(*maintenanceContextDriver).cleanupCalls)
}

func TestWorkflowContextRuntime_CanceledFailureUsesIndependentCleanupAndZeroizes(t *testing.T) {
	t.Parallel()
	request := newWorkflowContextRuntimeFixture(t)
	attachValidWorkflowContextPromotion(t, &request)
	store := promptlayer.NewOMPContextTransientStore()
	driver := &maintenanceContextDriver{emitCheckpointThenCancel: true}
	overlay := &maintenanceContextOverlay{activeFirst: true}
	request.Driver, request.Overlay = driver, overlay
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	receipt, err := NewWorkflowContextRuntimeSupervisor(store).Run(ctx, request, nil)

	require.Error(t, err)
	assert.Equal(t, WorkflowContextOutcomeBlocked, receipt.Outcome)
	assert.Equal(t, 1, driver.cleanupCalls)
	assert.False(t, driver.cleanupCanceled)
	assert.False(t, overlay.rollbackCanceled)
	assert.Zero(t, store.Pending())
}

func TestWorkflowContextRuntime_RejectsBodyLikeMetadataBeforeReceiptConstruction(t *testing.T) {
	t.Parallel()
	request := newWorkflowContextRuntimeFixture(t)
	request.Binding.WorkspaceID = "this is raw request body content"
	request.Driver = &maintenanceContextDriver{}
	request.Overlay = &maintenanceContextOverlay{}
	root := request.Binding.DeliveryOptions.Root
	request.ReceiptWriter = &WorkflowContextReceiptWriter{WorkspaceRoot: root}

	receipt, err := NewWorkflowContextRuntimeSupervisor(nil).Run(context.Background(), request, nil)

	require.Error(t, err)
	assert.Empty(t, receipt.SchemaVersion)
	path := filepath.Join(root, filepath.FromSlash(WorkflowContextReceiptRelativePath(request.Binding.TaskID, request.Binding.SessionID)))
	_, statErr := os.Stat(path)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
	assert.Equal(t, 1, request.Driver.(*maintenanceContextDriver).cleanupCalls)
}

type maintenanceContextDriver struct {
	mu                       sync.Mutex
	runCalls                 int
	cleanupCalls             int
	cleanupCanceled          bool
	emitCheckpointThenCancel bool
}

func (*maintenanceContextDriver) ArtifactCount(context.Context) (int, error) { return 0, nil }

func (driver *maintenanceContextDriver) Run(_ context.Context, emit func(WorkflowContextRuntimeEvent) error) error {
	driver.mu.Lock()
	driver.runCalls++
	driver.mu.Unlock()
	if driver.emitCheckpointThenCancel {
		if err := emit(WorkflowContextRuntimeEvent{Kind: WorkflowContextEventPreCompaction}); err != nil {
			return err
		}
		return context.Canceled
	}
	return nil
}

func (driver *maintenanceContextDriver) Cleanup(ctx context.Context) error {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	driver.cleanupCalls++
	driver.cleanupCanceled = ctx.Err() != nil
	return ctx.Err()
}

type maintenanceContextOverlay struct {
	mu               sync.Mutex
	activeFirst      bool
	applyCalls       int
	rollbackCanceled bool
}

func (overlay *maintenanceContextOverlay) Apply(ctx context.Context, request WorkflowContextOverlayRequest) (WorkflowContextOverlayReadback, error) {
	overlay.mu.Lock()
	defer overlay.mu.Unlock()
	overlay.applyCalls++
	if request.HistoryMode == config.OMPContextHistoryActive && overlay.activeFirst {
		return activeOverlayReadback(), nil
	}
	overlay.rollbackCanceled = ctx.Err() != nil
	return shadowOverlayReadback(), ctx.Err()
}
