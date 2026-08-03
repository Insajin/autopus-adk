package cli

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextProductOverlay_FailureRollsBackBeforeTaskCleanup(t *testing.T) {
	input, runtime, _, _ := newWorkflowContextProductFixture(t)
	controller, configPath, err := newWorkflowContextProductOverlay(
		runtime.DriverOptions.RuntimeRoot, config.OMPContextMemoryOff,
	)
	require.NoError(t, err)
	driver := &workflowContextProductCleanupOrderDriver{
		runtimeRoot: runtime.DriverOptions.RuntimeRoot,
		configPath:  configPath,
	}
	runtime.Overlay = controller
	runtime.DriverOptions.ConfigPath = configPath
	runtime.NewManagedDriver = func(WorkflowContextManagedRPCOptions) (WorkflowContextManagedProcessDriver, error) {
		return driver, nil
	}

	receipt, err := RunWorkflowContextProductSession(context.Background(), input, runtime)
	require.ErrorContains(t, err, "forced product runtime failure")
	assert.True(t, driver.cleanupSawShadow, "rollback readback must complete while the task overlay still exists")
	assert.Equal(t, 1, driver.cleanupCalls)
	assert.Equal(t, config.OMPContextHistoryShadow, receipt.Mode.EffectiveHistoryMode)
	assert.True(t, receipt.Cleanup.Attempted)
	assert.True(t, receipt.Cleanup.Verified)
	_, statErr := os.Lstat(runtime.DriverOptions.RuntimeRoot)
	assert.True(t, errors.Is(statErr, os.ErrNotExist))
}

type workflowContextProductCleanupOrderDriver struct {
	runtimeRoot      string
	configPath       string
	cleanupCalls     int
	cleanupSawShadow bool
}

func (driver *workflowContextProductCleanupOrderDriver) verifyInstalledIdentity(context.Context, string) error {
	return nil
}

func (driver *workflowContextProductCleanupOrderDriver) Bind(context.Context, WorkflowContextBridgeBinding) error {
	return nil
}

func (driver *workflowContextProductCleanupOrderDriver) Dispatch(
	context.Context, WorkflowContextDispatch,
) (WorkflowContextDispatchAck, error) {
	return WorkflowContextDispatchAck{}, errors.New("unexpected dispatch")
}

func (driver *workflowContextProductCleanupOrderDriver) ArtifactCount(context.Context) (int, error) {
	if _, err := os.Lstat(driver.runtimeRoot); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return 1, nil
}

func (driver *workflowContextProductCleanupOrderDriver) Run(
	_ context.Context, emit func(WorkflowContextRuntimeEvent) error,
) error {
	if err := emit(WorkflowContextRuntimeEvent{Kind: WorkflowContextEventPreCompaction}); err != nil {
		return err
	}
	return errors.New("forced product runtime failure")
}

func (driver *workflowContextProductCleanupOrderDriver) Cleanup(context.Context) error {
	driver.cleanupCalls++
	body, err := os.ReadFile(driver.configPath)
	if err != nil {
		return err
	}
	driver.cleanupSawShadow = strings.Contains(string(body), "compaction:\n  enabled: false")
	return os.RemoveAll(driver.runtimeRoot)
}
