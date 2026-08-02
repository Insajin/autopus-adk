package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextReceiptWriter_BodyFreeAtomic0600AndTaskOwned(t *testing.T) {
	t.Parallel()
	request := newWorkflowContextRuntimeFixture(t)
	request.Driver = &fakeWorkflowContextDriver{events: []WorkflowContextRuntimeEvent{
		{Kind: WorkflowContextEventPreCompaction},
		{Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 2}},
		{Kind: WorkflowContextEventPostCompaction},
	}, artifacts: 2}
	request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
	request.ReceiptWriter = &WorkflowContextReceiptWriter{WorkspaceRoot: request.Binding.DeliveryOptions.Root}

	receipt, err := NewWorkflowContextRuntimeSupervisor(nil).Run(context.Background(), request, (&recordingWorkflowContextDispatcher{}).Dispatch)
	require.NoError(t, err)
	rel := WorkflowContextReceiptRelativePath(receipt.TaskID, receipt.SessionID)
	path := filepath.Join(request.Binding.DeliveryOptions.Root, filepath.FromSlash(rel))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), request.Binding.Ephemeral.OriginalTask)
	assert.NotContains(t, string(raw), request.Binding.History[0].Body)
	assert.NotContains(t, string(raw), request.Binding.DeliveryOptions.Root)
	assert.NotContains(t, string(raw), "body")
	assert.NotContains(t, string(raw), "secret")
	assert.Contains(t, string(raw), `"frozen_finding_ids":["F-002","F-010"]`)
	assert.Contains(t, string(raw), `"worker_result_fields":["owned_paths","changed_files","verification","blockers","next_required_step"]`)
	var persisted WorkflowContextRuntimeReceipt
	require.NoError(t, json.Unmarshal(raw, &persisted))
	assert.Equal(t, receipt.BindingHash, persisted.BindingHash)
	assert.Empty(t, persisted.DocumentOmissions)
	assert.Empty(t, persisted.MemoryInjections)
}

func TestWorkflowContextReceiptWriter_RejectsSecretAbsolutePathAndSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := WorkflowContextReceiptWriter{WorkspaceRoot: root}
	receipt := WorkflowContextRuntimeReceipt{
		SchemaVersion: WorkflowContextRuntimeReceiptSchemaVersion, Event: "terminal", WorkspaceID: "workspace",
		SpecID: "SPEC-OMP-004", TaskID: "TASK-7", Phase: "go", SessionID: "session-1",
		RootClass:         config.OMPContextRuntimeIsolatedTaskOwned,
		Mode:              WorkflowContextModeReceipt{RequestedHistoryMode: config.OMPContextHistoryActive, EffectiveHistoryMode: config.OMPContextHistoryShadow},
		DocumentOmissions: []string{}, MemoryInjections: []string{},
	}
	receipt.Fallback.Reason = "token=sk-test-SECRET"
	require.Error(t, writer.Write(receipt))
	receipt.Fallback.Reason = "failed under /Users/example/private"
	require.Error(t, writer.Write(receipt))
	receipt.Fallback.Reason = "safe-reason"
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".autopus"), 0o700))
	require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(root, ".autopus", "runtime")))
	require.Error(t, writer.Write(receipt))
}

func TestWorkflowContextInstalledCanary_RequiresObservedLoopbackCapability(t *testing.T) {
	t.Parallel()
	request := newWorkflowContextRuntimeFixture(t)
	driver := &fakeWorkflowContextDriver{events: []WorkflowContextRuntimeEvent{
		{Kind: WorkflowContextEventPreCompaction}, {Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 2}}, {Kind: WorkflowContextEventPostCompaction},
	}, artifacts: 1}
	request.Driver = driver
	request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
	dispatcher := &recordingWorkflowContextDispatcher{}
	receipt, err := RunWorkflowContextInstalledCanary(context.Background(), NewWorkflowContextRuntimeSupervisor(nil), request, dispatcher.Dispatch)
	require.NoError(t, err)
	assert.Equal(t, "omp/17.1.8", receipt.Capabilities.Version)
	assert.Equal(t, 1, dispatcher.optimized)

	request = newWorkflowContextRuntimeFixture(t)
	request.Capabilities.AuthNoneLoopback = false
	request.Driver = &fakeWorkflowContextDriver{}
	_, err = RunWorkflowContextInstalledCanary(context.Background(), NewWorkflowContextRuntimeSupervisor(nil), request, (&recordingWorkflowContextDispatcher{}).Dispatch)
	require.ErrorContains(t, err, "auth:none loopback")
}

func TestWorkflowCommand_RegistersContextRuntimePolicyInspection(t *testing.T) {
	t.Parallel()
	cmd := NewWorkflowCmd(nil, nil)
	found, _, err := cmd.Find([]string{"context-runtime"})
	require.NoError(t, err)
	assert.Equal(t, "context-runtime", found.Name())
}
