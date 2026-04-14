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
	"github.com/insajin/autopus-adk/pkg/worker/budget"
	"github.com/insajin/autopus-adk/pkg/worker/routing"
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

// --- Mock adapter for integration tests ---

// mockAdapter implements ProviderAdapter using a helper script for testing.
type mockAdapter struct {
	name      string
	script    string // shell script content for subprocess
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

func TestExecuteSubprocess_CodexJSONTurnCompletion(t *testing.T) {
	script := `head -c0; echo '{"type":"thread.started","thread_id":"t1"}'; echo '{"type":"turn.started"}'; echo '{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"task done via codex"}}'; echo '{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":5}}'`
	codex := adapter.NewCodexAdapter()
	mock := &mockAdapter{
		name:      "codex-mock",
		script:    script,
		parseFn:   codex.ParseEvent,
		extractFn: codex.ExtractResult,
	}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock},
	}

	taskCfg := adapter.TaskConfig{
		TaskID: "test-codex-v2",
		Prompt: "do work",
	}

	result, err := wl.executeSubprocess(context.Background(), taskCfg)
	require.NoError(t, err)
	assert.Equal(t, "task done via codex", result.Output)
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
	require.Len(t, result.Artifacts, 1)
	assert.Equal(t, "output", result.Artifacts[0].Name)
	assert.Equal(t, "done", result.Artifacts[0].Data)
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

func TestHandleTask_PrefersBackendSelectedModel(t *testing.T) {
	script := `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`
	mock := &mockAdapter{name: "mock", script: script}
	router := routing.NewRouter(routing.RoutingConfig{
		Enabled: true,
		Thresholds: routing.ClassifierThresholds{
			SimpleMaxChars:  10,
			ComplexMinChars: 20,
		},
		Models: map[string]routing.ProviderModels{
			"mock": {Simple: "local-simple", Medium: "local-medium", Complex: "local-complex"},
		},
	})

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock, WorkDir: t.TempDir(), Router: router},
	}

	payload, _ := json.Marshal(taskPayloadMessage{
		Description: "this description would route locally",
		Model:       "server-selected-model",
	})

	result, err := wl.handleTask(context.Background(), "task-ht-model", payload)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "server-selected-model", mock.last.Model)
}

func TestHandleTask_DisablesLocalRoutingWhenSignedControlPlaneEnabled(t *testing.T) {
	t.Setenv(a2a.PolicySigningSecretEnv, "test-secret")

	script := `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`
	mock := &mockAdapter{name: "mock", script: script}
	router := routing.NewRouter(routing.RoutingConfig{
		Enabled: true,
		Thresholds: routing.ClassifierThresholds{
			SimpleMaxChars:  10,
			ComplexMinChars: 20,
		},
		Models: map[string]routing.ProviderModels{
			"mock": {Simple: "local-simple", Medium: "local-medium", Complex: "local-complex"},
		},
	})

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock, WorkDir: t.TempDir(), Router: router},
	}

	payload, _ := json.Marshal(taskPayloadMessage{
		Description: "this description would route locally",
	})

	result, err := wl.handleTask(context.Background(), "task-ht-no-local-routing", payload)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, mock.last.Model)
}

func TestHandleTask_UsesBackendSelectedPipelinePhases(t *testing.T) {
	script := `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`
	mock := &mockAdapter{name: "mock", script: script}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock, WorkDir: t.TempDir()},
	}

	payload, _ := json.Marshal(taskPayloadMessage{
		Prompt:         "backend-built prompt",
		PipelinePhases: []string{"planner", "reviewer"},
		Model:          "server-selected-model",
	})

	result, err := wl.handleTask(context.Background(), "task-ht-pipeline", payload)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, mock.calls, 2)
	assert.Equal(t, "task-ht-pipeline-planner", mock.calls[0].TaskID)
	assert.Equal(t, "task-ht-pipeline-reviewer", mock.calls[1].TaskID)
	assert.Equal(t, "server-selected-model", mock.calls[0].Model)
	assert.Equal(t, "server-selected-model", mock.calls[1].Model)
}

func TestHandleTask_UsesBackendSelectedPipelineInstructions(t *testing.T) {
	script := `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`
	mock := &mockAdapter{name: "mock", script: script}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock, WorkDir: t.TempDir()},
	}

	payload, _ := json.Marshal(taskPayloadMessage{
		Prompt:         "backend-built prompt",
		PipelinePhases: []string{"planner"},
		PipelineInstructions: map[string]string{
			"planner": "Use the backend-selected planning instruction.",
		},
	})

	result, err := wl.handleTask(context.Background(), "task-ht-pipeline-instructions", payload)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, mock.calls, 1)
	assert.Contains(t, mock.calls[0].Prompt, "backend-selected planning instruction")
	assert.Contains(t, mock.calls[0].Prompt, "backend-built prompt")
}

func TestHandleTask_UsesBackendSelectedPipelinePromptTemplates(t *testing.T) {
	script := `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`
	mock := &mockAdapter{name: "mock", script: script}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock, WorkDir: t.TempDir()},
	}

	payload, _ := json.Marshal(taskPayloadMessage{
		Prompt:         "backend-built prompt",
		PipelinePhases: []string{"planner"},
		PipelinePromptTemplates: map[string]string{
			"planner": "SERVER TEMPLATE\n\n{{input}}",
		},
	})

	result, err := wl.handleTask(context.Background(), "task-ht-pipeline-template", payload)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, mock.calls, 1)
	assert.Contains(t, mock.calls[0].Prompt, "SERVER TEMPLATE")
	assert.Contains(t, mock.calls[0].Prompt, "backend-built prompt")
}

func TestHandleTask_EnforcesServerIssuedIterationBudget(t *testing.T) {
	script := `head -c0; echo '{"type":"tool_call"}'; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`
	mock := &mockAdapter{name: "mock", script: script}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock, WorkDir: t.TempDir()},
	}

	payload, _ := json.Marshal(taskPayloadMessage{
		Prompt: "backend-built prompt",
		IterationBudget: &budget.IterationBudget{
			Limit:           1,
			WarnThreshold:   0.70,
			DangerThreshold: 0.90,
		},
	})

	_, err := wl.handleTask(context.Background(), "task-ht-budget", payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iteration budget exceeded")
}

func TestHandleTask_InvalidPipelineInstructions(t *testing.T) {
	mock := &mockAdapter{name: "mock", script: "true"}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock, WorkDir: t.TempDir()},
	}

	payload, _ := json.Marshal(taskPayloadMessage{
		Description: "test task",
		PipelineInstructions: map[string]string{
			"deployer": "ship it",
		},
	})

	_, err := wl.handleTask(context.Background(), "task-invalid-instruction", payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported phase")
}

func TestHandleTask_InvalidPipelinePhase(t *testing.T) {
	mock := &mockAdapter{name: "mock", script: "true"}

	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock, WorkDir: t.TempDir()},
	}

	payload, _ := json.Marshal(taskPayloadMessage{
		Description:    "test task",
		PipelinePhases: []string{"planner", "deployer"},
	})

	_, err := wl.handleTask(context.Background(), "task-invalid-phase", payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported phase")
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

func TestPrepareTaskRuntimeEnv_ConfiguresWritableCaches(t *testing.T) {
	taskCfg := adapter.TaskConfig{WorkDir: t.TempDir()}

	err := prepareTaskRuntimeEnv(&taskCfg)
	require.NoError(t, err)

	assert.DirExists(t, filepath.Join(taskCfg.WorkDir, ".symphony", "artifacts", "tmp"))
	assert.DirExists(t, filepath.Join(taskCfg.WorkDir, ".symphony", "artifacts", "gocache"))
	assert.Equal(t, filepath.Join(taskCfg.WorkDir, ".symphony", "artifacts", "tmp"), taskCfg.EnvVars["TMPDIR"])
	assert.Equal(t, filepath.Join(taskCfg.WorkDir, ".symphony", "artifacts", "tmp"), taskCfg.EnvVars["TEST_TMPDIR"])
	assert.Equal(t, filepath.Join(taskCfg.WorkDir, ".symphony", "artifacts", "tmp"), taskCfg.EnvVars["GOTMPDIR"])
	assert.Equal(t, filepath.Join(taskCfg.WorkDir, ".symphony", "artifacts", "gocache"), taskCfg.EnvVars["GOCACHE"])
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
