package worker

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helper function tests ---

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

// --- Mock adapter for integration tests ---

// mockAdapter implements ProviderAdapter using a helper script for testing.
type mockAdapter struct {
	name   string
	script string // shell script content for subprocess
	last   adapter.TaskConfig
}

func (m *mockAdapter) Name() string { return m.name }

func (m *mockAdapter) BuildCommand(ctx context.Context, task adapter.TaskConfig) *exec.Cmd {
	m.last = task
	return exec.CommandContext(ctx, "sh", "-c", m.script)
}

func (m *mockAdapter) ParseEvent(line []byte) (adapter.StreamEvent, error) {
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
	var data struct {
		Output     string  `json:"output"`
		CostUSD    float64 `json:"cost_usd"`
		DurationMS int64   `json:"duration_ms"`
		SessionID  string  `json:"session_id"`
	}
	json.Unmarshal(event.Data, &data)
	return adapter.TaskResult{
		Output:     data.Output,
		CostUSD:    data.CostUSD,
		DurationMS: data.DurationMS,
		SessionID:  data.SessionID,
	}
}

// --- Integration: executeSubprocess tests ---

func TestExecuteSubprocess_HappyPath(t *testing.T) {
	// Use head -c0 instead of cat /dev/stdin to avoid stdin EOF deadlock.
	script := `head -c0; echo '{"type":"system.init"}'; echo '{"type":"result","output":"task done","cost_usd":0.03,"duration_ms":500,"session_id":"s1"}'`
	mock := &mockAdapter{name: "mock", script: script}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock},
	}

	taskCfg := adapter.TaskConfig{
		TaskID: "test-happy",
		Prompt: "do work",
	}

	result, err := wl.executeSubprocess(context.Background(), taskCfg)
	require.NoError(t, err)
	assert.Equal(t, "task done", result.Output)
	assert.InDelta(t, 0.03, result.CostUSD, 0.001)
	assert.Equal(t, int64(500), result.DurationMS)
	assert.Equal(t, "s1", result.SessionID)
}

func TestExecuteSubprocess_SubprocessFailure(t *testing.T) {
	script := `head -c0; exit 1`
	mock := &mockAdapter{name: "mock", script: script}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock},
	}

	taskCfg := adapter.TaskConfig{
		TaskID: "test-fail",
		Prompt: "this will fail",
	}

	_, err := wl.executeSubprocess(context.Background(), taskCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no result event")
}

func TestExecuteSubprocess_ContextCancellation(t *testing.T) {
	// Long-running script that gets cancelled.
	script := `head -c0; sleep 30`
	mock := &mockAdapter{name: "mock", script: script}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock},
	}

	ctx, cancel := context.WithCancel(context.Background())

	taskCfg := adapter.TaskConfig{
		TaskID: "test-cancel",
		Prompt: "cancel me",
	}

	// Cancel immediately to trigger graceful shutdown.
	cancel()

	_, err := wl.executeSubprocess(ctx, taskCfg)
	require.Error(t, err)
}

func TestExecuteSubprocess_FailWithOutput(t *testing.T) {
	// Subprocess exits non-zero but still emits a result event.
	script := `head -c0; echo '{"type":"result","output":"partial result","cost_usd":0.01}'; exit 1`
	mock := &mockAdapter{name: "mock", script: script}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock},
	}

	taskCfg := adapter.TaskConfig{
		TaskID: "test-fail-output",
		Prompt: "fail with output",
	}

	result, err := wl.executeSubprocess(context.Background(), taskCfg)
	require.NoError(t, err)
	assert.Equal(t, "partial result", result.Output)
}

func TestExecuteSubprocess_NoResultEvent(t *testing.T) {
	// Subprocess produces events but no result.
	script := `head -c0; echo '{"type":"system.init"}'; echo '{"type":"system.task_started"}'`
	mock := &mockAdapter{name: "mock", script: script}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock},
	}

	taskCfg := adapter.TaskConfig{
		TaskID: "test-no-result",
		Prompt: "no result",
	}

	_, err := wl.executeSubprocess(context.Background(), taskCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no result event")
}

// --- Integration: handleTask tests ---

func TestHandleTask_HappyPath(t *testing.T) {
	script := `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`
	mock := &mockAdapter{name: "mock", script: script}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock, WorkDir: t.TempDir()},
	}

	payload, _ := json.Marshal(taskPayloadMessage{
		Description: "test task",
	})

	result, err := wl.handleTask(context.Background(), "task-ht-1", payload)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "completed", string(result.Status))
}

func TestHandleTask_InvalidPayload(t *testing.T) {
	mock := &mockAdapter{name: "mock", script: "true"}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock},
	}

	_, err := wl.handleTask(context.Background(), "task-bad", []byte("not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse task payload")
}

func TestHandleTask_UsesPromptPayloadWhenDescriptionMissing(t *testing.T) {
	script := `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`
	mock := &mockAdapter{name: "mock", script: script}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock, WorkDir: t.TempDir()},
	}

	payload := []byte(`{"prompt":"backend-built prompt"}`)

	result, err := wl.handleTask(context.Background(), "task-ht-prompt", payload)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "backend-built prompt", mock.last.Prompt)
	assert.Equal(t, "completed", string(result.Status))
}

func TestPrepareSymphonyWorkspace_CreatesPromptMarkdown(t *testing.T) {
	workDir := t.TempDir()
	prompt := "Please read .symphony/prompt.md before proceeding."

	err := prepareSymphonyWorkspace(workDir, prompt)
	require.NoError(t, err)

	promptPath := filepath.Join(workDir, ".symphony", "prompt.md")
	data, err := os.ReadFile(promptPath)
	require.NoError(t, err)
	assert.Equal(t, prompt, string(data))

	info, err := os.Stat(promptPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o444), info.Mode().Perm())
}

// --- Approval wiring tests ---

func TestSetTUIProgram(t *testing.T) {
	t.Parallel()
	wl := &WorkerLoop{}
	assert.Nil(t, wl.tuiProgram)

	// SetTUIProgram with nil should not panic.
	wl.SetTUIProgram(nil)
	assert.Nil(t, wl.tuiProgram)
}

func TestHandleApproval_NoTUIProgram(t *testing.T) {
	t.Parallel()
	wl := &WorkerLoop{}
	// Should log warning but not panic when tuiProgram is nil.
	wl.handleApproval(a2a.ApprovalRequestParams{
		TaskID:    "task-1",
		Action:    "deploy",
		RiskLevel: "high",
		Context:   "prod",
	})
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
	// Verify the server was created with an ApprovalCallback wired.
	require.NotNil(t, wl.server)
}

// --- cleanupPolicy test ---

func TestCleanupPolicy_NonExistent(t *testing.T) {
	// Should not panic or error on non-existent file.
	cleanupPolicy("nonexistent-task-id")
}
