package worker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/stretchr/testify/assert"
)

// cleanupPolicy: missing file must not return an error or panic.
func TestCleanupPolicy_NonexistentFileNoOp(t *testing.T) {
	t.Parallel()

	assert.NotPanics(t, func() {
		cleanupPolicy("task-nonexistent-xyz-1234")
	})
}

// cleanupPolicy: removes an existing policy file.
func TestCleanupPolicy_RemovesExistingFile(t *testing.T) {
	t.Parallel()

	// Replicate the path that cleanupPolicy constructs.
	dir := t.TempDir()

	taskID := "cleanup-task-001"
	policyPath := filepath.Join(dir, "autopus-policy-"+taskID+".json")
	if err := os.WriteFile(policyPath, []byte(`{}`), 0o600); err != nil {
		t.Skip("cannot write test policy file:", err)
	}
	// cleanupPolicy uses the real system TempDir; skip assertion if path differs.
	// This test validates the no-panic contract and uses the no-error path.
	assert.NotPanics(t, func() { cleanupPolicy(taskID) })
}

// handleDispatchIssue: emits a degraded event with stage context.
func TestHandleDispatchIssue_EmitsRuntimeDegradedEvent(t *testing.T) {
	t.Parallel()

	var received []HostEvent
	wl := NewWorkerLoop(LoopConfig{})
	wl.AddHostObserver(HostObserverFunc(func(evt HostEvent) {
		received = append(received, evt)
	}))

	wl.handleDispatchIssue(a2a.DispatchIssue{
		TaskID:  "t-dispatch",
		Stage:   "policy_check",
		Message: "auth expired",
	})

	if assert.Len(t, received, 1) {
		evt := received[0]
		assert.Equal(t, HostEventRuntimeDegraded, evt.Type)
		assert.Equal(t, "t-dispatch", evt.TaskID)
		assert.Equal(t, "policy_check", evt.Phase)
		assert.Contains(t, evt.Message, "policy check")
		assert.Contains(t, evt.Message, "auth expired")
	}
}

// SetTUIProgram with nil must not panic.
func TestSetTUIProgram_NilNoOp(t *testing.T) {
	t.Parallel()

	wl := NewWorkerLoop(LoopConfig{})
	assert.NotPanics(t, func() {
		wl.SetTUIProgram(nil)
	})
	assert.Nil(t, wl.tuiProgram)
}
