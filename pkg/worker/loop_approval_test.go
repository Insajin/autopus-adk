package worker

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetTUIProgram(t *testing.T) {
	t.Parallel()
	wl := &WorkerLoop{}
	assert.Nil(t, wl.tuiProgram)

	wl.SetTUIProgram(nil)
	assert.Nil(t, wl.tuiProgram)
}

func TestHandleApproval_NoTUIProgram(t *testing.T) {
	t.Parallel()
	wl := &WorkerLoop{}
	wl.handleApproval(a2a.ApprovalRequestParams{
		TaskID:    "task-1",
		Action:    "deploy",
		RiskLevel: "high",
		Context:   "prod",
	})
}

func TestHandleApproval_NotifiesObserver(t *testing.T) {
	t.Parallel()

	events := make([]HostEvent, 0, 1)
	wl := &WorkerLoop{}
	wl.AddHostObserver(HostObserverFunc(func(event HostEvent) {
		events = append(events, event)
	}))

	wl.handleApproval(a2a.ApprovalRequestParams{
		TaskID:    "task-1",
		Action:    "deploy",
		RiskLevel: "high",
		Context:   "prod",
	})

	require.Len(t, events, 1)
	assert.Equal(t, HostEventApprovalRequested, events[0].Type)
	assert.Equal(t, "task-1", events[0].TaskID)
}

func TestSetOnApprovalDecision_ReturnsCallback(t *testing.T) {
	t.Parallel()
	wl := &WorkerLoop{}
	cb := wl.SetOnApprovalDecision()
	assert.NotNil(t, cb)
}

func TestNewWorkerLoop_WiresApprovalCallback(t *testing.T) {
	t.Parallel()
	cfg := LoopConfig{
		BackendURL: "http://localhost:8080",
		WorkerName: "test-worker",
		Skills:     []string{"code"},
		Provider:   adapter.NewClaudeAdapter(),
		WorkDir:    "/tmp",
	}
	wl := NewWorkerLoop(cfg)
	require.NotNil(t, wl.server)
}

func TestCleanupPolicy_NonExistent(t *testing.T) {
	cleanupPolicy("nonexistent-task-id")
}
