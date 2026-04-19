package worker

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pipelineMockAdapter implements ProviderAdapter using a shell script.
type pipelineMockAdapter struct {
	script    string
	parseFn   func([]byte) (adapter.StreamEvent, error)
	extractFn func(adapter.StreamEvent) adapter.TaskResult
	calls     []adapter.TaskConfig
}

func (m *pipelineMockAdapter) Name() string { return "pipeline-mock" }

func (m *pipelineMockAdapter) BuildCommand(ctx context.Context, task adapter.TaskConfig) *exec.Cmd {
	m.calls = append(m.calls, task)
	return exec.CommandContext(ctx, "sh", "-c", m.script)
}

func (m *pipelineMockAdapter) ParseEvent(line []byte) (adapter.StreamEvent, error) {
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

func (m *pipelineMockAdapter) ExtractResult(event adapter.StreamEvent) adapter.TaskResult {
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

func TestPipelineExecute_HappyPath(t *testing.T) {
	script := `cat /dev/stdin > /dev/null; echo '{"type":"result","output":"phase ok","cost_usd":0.01,"duration_ms":100}'`
	mock := &pipelineMockAdapter{script: script}

	pe := NewPipelineExecutor(mock, "", t.TempDir())
	result, err := pe.Execute(context.Background(), "pipe-1", "initial prompt")

	require.NoError(t, err)
	assert.InDelta(t, 0.04, result.CostUSD, 0.001)
	assert.Equal(t, int64(400), result.DurationMS)
	for _, phase := range []string{"planner", "executor", "tester", "reviewer"} {
		assert.True(t, strings.Contains(result.Output, phase),
			"output should contain phase %q", phase)
	}
}

func TestPipelineExecute_PhaseFailure(t *testing.T) {
	script := `cat /dev/stdin > /dev/null; exit 1`
	mock := &pipelineMockAdapter{script: script}

	pe := NewPipelineExecutor(mock, "", t.TempDir())
	_, err := pe.Execute(context.Background(), "pipe-fail", "prompt")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "phase planner")
}

func TestPipelineExecute_ContextCancel(t *testing.T) {
	script := `cat /dev/stdin > /dev/null; sleep 30`
	mock := &pipelineMockAdapter{script: script}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pe := NewPipelineExecutor(mock, "", t.TempDir())
	_, err := pe.Execute(ctx, "pipe-cancel", "prompt")

	require.Error(t, err)
}

func TestPipelineExecute_MissingDefaultPhasePlanInSignedMode(t *testing.T) {
	t.Setenv("AUTOPUS_A2A_POLICY_SIGNING_SECRET", "test-secret")

	script := `cat /dev/stdin > /dev/null; echo '{"type":"result","output":"phase ok","cost_usd":0.01,"duration_ms":100}'`
	mock := &pipelineMockAdapter{script: script}

	pe := NewPipelineExecutor(mock, "", t.TempDir())
	_, err := pe.Execute(context.Background(), "pipe-signed-no-plan", "initial prompt")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing pipeline phase plan")
}

func TestPipelineExecuteWithPlan_UsesServerSelectedPhaseOrder(t *testing.T) {
	script := `cat /dev/stdin > /dev/null; echo '{"type":"result","output":"phase ok","cost_usd":0.01,"duration_ms":100}'`
	mock := &pipelineMockAdapter{script: script}

	pe := NewPipelineExecutor(mock, "", t.TempDir())
	result, err := pe.ExecuteWithPlan(context.Background(), "pipe-plan", "initial prompt", "server-model", []Phase{PhasePlanner, PhaseReviewer})

	require.NoError(t, err)
	require.Len(t, mock.calls, 2)
	assert.Equal(t, "server-model", mock.calls[0].Model)
	assert.Equal(t, "server-model", mock.calls[1].Model)
	assert.Equal(t, "pipe-plan-planner", mock.calls[0].TaskID)
	assert.Equal(t, "pipe-plan-reviewer", mock.calls[1].TaskID)
	assert.Contains(t, result.Output, "planner")
	assert.Contains(t, result.Output, "reviewer")
	assert.NotContains(t, result.Output, "executor")
	assert.NotContains(t, result.Output, "tester")
}

func TestPipelineExecuteWithPlan_UsesServerSelectedPhaseInstructions(t *testing.T) {
	script := `cat /dev/stdin > /dev/null; echo '{"type":"result","output":"phase ok","cost_usd":0.01,"duration_ms":100}'`
	mock := &pipelineMockAdapter{script: script}

	pe := NewPipelineExecutor(mock, "", t.TempDir())
	pe.SetPhaseInstructions(map[Phase]string{
		PhasePlanner: "Use the server-provided planning instruction.",
	})

	_, err := pe.ExecuteWithPlan(context.Background(), "pipe-instruction", "initial prompt", "server-model", []Phase{PhasePlanner})

	require.NoError(t, err)
	require.Len(t, mock.calls, 1)
	assert.Contains(t, mock.calls[0].Prompt, "server-provided planning instruction")
	assert.Contains(t, mock.calls[0].Prompt, "initial prompt")
}

func TestPipelineExecuteWithPlan_UsesServerSelectedPromptTemplates(t *testing.T) {
	script := `cat /dev/stdin > /dev/null; echo '{"type":"result","output":"phase ok","cost_usd":0.01,"duration_ms":100}'`
	mock := &pipelineMockAdapter{script: script}

	pe := NewPipelineExecutor(mock, "", t.TempDir())
	pe.SetPhasePromptTemplates(map[Phase]string{
		PhasePlanner: "SERVER TEMPLATE\n\n{{input}}",
	})

	_, err := pe.ExecuteWithPlan(context.Background(), "pipe-template", "initial prompt", "server-model", []Phase{PhasePlanner})

	require.NoError(t, err)
	require.Len(t, mock.calls, 1)
	assert.Contains(t, mock.calls[0].Prompt, "SERVER TEMPLATE")
	assert.Contains(t, mock.calls[0].Prompt, "initial prompt")
}

func TestPipelineRunPhase_NoResultEvent(t *testing.T) {
	script := `cat /dev/stdin > /dev/null; echo '{"type":"system.init"}'`
	mock := &pipelineMockAdapter{script: script}

	pe := &PipelineExecutor{provider: mock, workDir: t.TempDir()}
	_, err := pe.runPhase(context.Background(), "task-x", PhasePlanner, "prompt", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no result event")
}

func TestPipelineRunPhase_FailWithOutput(t *testing.T) {
	// Exits non-zero but has a result event.
	script := `cat /dev/stdin > /dev/null; echo '{"type":"result","output":"partial"}'; exit 1`
	mock := &pipelineMockAdapter{script: script}

	pe := &PipelineExecutor{provider: mock, workDir: t.TempDir()}
	result, err := pe.runPhase(context.Background(), "task-y", PhaseExecutor, "prompt", "")

	require.NoError(t, err)
	assert.Equal(t, "partial", result.Output)
	assert.Equal(t, PhaseExecutor, result.Phase)
}

func TestPipelineRunPhase_CodexJSONTurnCompletion(t *testing.T) {
	script := `cat /dev/stdin > /dev/null; echo '{"type":"thread.started","thread_id":"t1"}'; echo '{"type":"turn.started"}'; echo '{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"phase ok"}}'; echo '{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":5}}'`
	codex := adapter.NewCodexAdapter()
	mock := &pipelineMockAdapter{
		script:    script,
		parseFn:   codex.ParseEvent,
		extractFn: codex.ExtractResult,
	}

	pe := &PipelineExecutor{provider: mock, workDir: t.TempDir()}
	result, err := pe.runPhase(context.Background(), "pipe-codex", PhasePlanner, "prompt", "")

	require.NoError(t, err)
	assert.Equal(t, "phase ok", result.Output)
	assert.Equal(t, PhasePlanner, result.Phase)
}

func TestPipelineExecuteWithPlan_DisablesLocalRoutingWhenSignedControlPlaneEnabled(t *testing.T) {
	t.Setenv("AUTOPUS_A2A_POLICY_SIGNING_SECRET", "test-secret")

	script := `cat /dev/stdin > /dev/null; echo '{"type":"result","output":"phase ok","cost_usd":0.01,"duration_ms":100}'`
	mock := &pipelineMockAdapter{script: script}

	pe := NewPipelineExecutor(mock, "", t.TempDir())
	pe.SetRouter(routing.NewRouter(enabledRoutingConfig()))

	_, err := pe.ExecuteWithPlan(context.Background(), "pipe-no-local-routing", "short prompt", "", []Phase{PhasePlanner})

	require.NoError(t, err)
	require.Len(t, mock.calls, 1)
	assert.Empty(t, mock.calls[0].Model)
}
