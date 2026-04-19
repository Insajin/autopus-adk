package worker

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertArtifacts_Empty(t *testing.T) {
	result := convertArtifacts(nil)
	assert.Nil(t, result)
}

func TestConvertArtifacts_Multiple(t *testing.T) {
	src := []adapter.Artifact{
		{Name: "a.txt", MimeType: "text/plain", Data: "hello"},
		{Name: "b.json", MimeType: "application/json", Data: "{}"},
	}
	result := convertArtifacts(src)
	require.Len(t, result, 2)
	assert.Equal(t, "a.txt", result[0].Name)
	assert.Equal(t, "{}", result[1].Data)
}

func TestEnsureOutputArtifact_AddsOutputArtifactFirst(t *testing.T) {
	artifacts := ensureOutputArtifact("worker summary", []adapter.Artifact{
		{Name: "notes.md", MimeType: "text/markdown", Data: "# notes"},
	})

	require.Len(t, artifacts, 2)
	assert.Equal(t, "output", artifacts[0].Name)
	assert.Equal(t, "worker summary", artifacts[0].Data)
	assert.Equal(t, "notes.md", artifacts[1].Name)
}

func TestEnsureOutputArtifact_DoesNotDuplicateExistingOutput(t *testing.T) {
	artifacts := ensureOutputArtifact("worker summary", []adapter.Artifact{
		{Name: "output", MimeType: "text/plain", Data: "existing summary"},
	})

	require.Len(t, artifacts, 1)
	assert.Equal(t, "existing summary", artifacts[0].Data)
}

func TestNewWorkerLoop(t *testing.T) {
	cfg := LoopConfig{
		BackendURL: "http://localhost:8080",
		WorkerName: "test-worker",
		Skills:     []string{"code"},
		Provider:   adapter.NewClaudeAdapter(),
		WorkDir:    "/tmp",
	}
	wl := NewWorkerLoop(cfg)
	require.NotNil(t, wl)
	assert.Equal(t, "test-worker", wl.config.WorkerName)
}

func TestConfigureExecutionConcurrency_SequentialStillInitializesSemaphore(t *testing.T) {
	wl := NewWorkerLoop(LoopConfig{
		Provider:       adapter.NewClaudeAdapter(),
		WorkDir:        t.TempDir(),
		MaxConcurrency: 1,
	})

	wl.configureExecutionConcurrency()

	require.NotNil(t, wl.semaphore)
	assert.Equal(t, 1, wl.semaphore.Limit())
	assert.Nil(t, wl.worktreeManager)
}

func TestConfigureExecutionConcurrency_ParallelEnablesWorktreeIsolation(t *testing.T) {
	wl := NewWorkerLoop(LoopConfig{
		Provider:          adapter.NewClaudeAdapter(),
		WorkDir:           t.TempDir(),
		MaxConcurrency:    3,
		WorktreeIsolation: true,
	})

	wl.configureExecutionConcurrency()

	require.NotNil(t, wl.semaphore)
	assert.Equal(t, 3, wl.semaphore.Limit())
	require.NotNil(t, wl.worktreeManager)
}

func TestDetachedTaskContext_IgnoresParentDeadline(t *testing.T) {
	wl := NewWorkerLoop(LoopConfig{Provider: adapter.NewClaudeAdapter()})

	parent, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	taskCtx, taskCancel := wl.detachedTaskContext(parent)
	defer taskCancel()

	time.Sleep(40 * time.Millisecond)
	require.ErrorIs(t, parent.Err(), context.DeadlineExceeded)
	select {
	case <-taskCtx.Done():
		t.Fatal("detached task context should ignore parent deadline")
	default:
	}
}

func TestDetachedTaskContext_PropagatesExplicitCancel(t *testing.T) {
	wl := NewWorkerLoop(LoopConfig{Provider: adapter.NewClaudeAdapter()})

	parent, cancel := context.WithCancel(context.Background())
	taskCtx, taskCancel := wl.detachedTaskContext(parent)
	defer taskCancel()

	cancel()

	select {
	case <-taskCtx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("detached task context should honor explicit cancellation")
	}
}

func TestTaskExecutionTimeout_ReadsCachedPolicy(t *testing.T) {
	wl := NewWorkerLoop(LoopConfig{Provider: adapter.NewClaudeAdapter()})
	cache := security.NewPolicyCache()
	taskID := "loop-timeout-test"
	require.NoError(t, cache.Write(taskID, security.SecurityPolicy{TimeoutSec: 123}))
	t.Cleanup(func() { cache.Delete(taskID) })

	timeout := wl.taskExecutionTimeout(taskID)
	assert.Equal(t, 123*time.Second, timeout)
}

// mockAdapter implements ProviderAdapter using a helper script for testing.
type mockAdapter struct {
	name      string
	script    string
	last      adapter.TaskConfig
	calls     []adapter.TaskConfig
	parseFn   func([]byte) (adapter.StreamEvent, error)
	extractFn func(adapter.StreamEvent) adapter.TaskResult
}

func (m *mockAdapter) Name() string { return m.name }

func (m *mockAdapter) BuildCommand(ctx context.Context, task adapter.TaskConfig) *exec.Cmd {
	m.last = task
	m.calls = append(m.calls, task)
	return exec.CommandContext(ctx, "sh", "-c", m.script)
}

func (m *mockAdapter) ParseEvent(line []byte) (adapter.StreamEvent, error) {
	if m.parseFn != nil {
		return m.parseFn(line)
	}
	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return adapter.StreamEvent{}, err
	}
	return adapter.StreamEvent{
		Type: raw.Type,
		Data: json.RawMessage(append([]byte(nil), line...)),
	}, nil
}

func (m *mockAdapter) ExtractResult(event adapter.StreamEvent) adapter.TaskResult {
	if m.extractFn != nil {
		return m.extractFn(event)
	}
	var data struct {
		Output     string  `json:"output"`
		CostUSD    float64 `json:"cost_usd"`
		DurationMS int64   `json:"duration_ms"`
		SessionID  string  `json:"session_id"`
	}
	if err := json.Unmarshal(event.Data, &data); err != nil {
		panic(err)
	}
	return adapter.TaskResult{
		Output:     data.Output,
		CostUSD:    data.CostUSD,
		DurationMS: data.DurationMS,
		SessionID:  data.SessionID,
	}
}
