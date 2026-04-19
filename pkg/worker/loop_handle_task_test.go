package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/insajin/autopus-adk/pkg/worker/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleTask_HappyPath(t *testing.T) {
	mock := &mockAdapter{name: "mock", script: `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`}
	wl := &WorkerLoop{config: LoopConfig{Provider: mock, WorkDir: t.TempDir()}}

	payload, _ := json.Marshal(taskPayloadMessage{Description: "test task"})
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
	wl := &WorkerLoop{config: LoopConfig{Provider: mock}}

	_, err := wl.handleTask(context.Background(), "task-bad", []byte("not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse task payload")
}

func TestHandleTask_UsesPromptPayloadWhenDescriptionMissing(t *testing.T) {
	mock := &mockAdapter{name: "mock", script: `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`}
	wl := &WorkerLoop{config: LoopConfig{Provider: mock, WorkDir: t.TempDir()}}

	result, err := wl.handleTask(context.Background(), "task-ht-prompt", []byte(`{"prompt":"backend-built prompt"}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "backend-built prompt", mock.last.Prompt)
	assert.Equal(t, "completed", string(result.Status))
}

func TestHandleTask_PrefersBackendSelectedModel(t *testing.T) {
	mock := &mockAdapter{name: "mock", script: `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`}
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

	wl := &WorkerLoop{config: LoopConfig{Provider: mock, WorkDir: t.TempDir(), Router: router}}
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

	mock := &mockAdapter{name: "mock", script: `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`}
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

	wl := &WorkerLoop{config: LoopConfig{Provider: mock, WorkDir: t.TempDir(), Router: router}}
	payload, _ := json.Marshal(taskPayloadMessage{
		Description: "this description would route locally",
	})

	result, err := wl.handleTask(context.Background(), "task-ht-no-local-routing", payload)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, mock.last.Model)
}

func TestHandleTask_UsesBackendSelectedPipelinePhases(t *testing.T) {
	mock := &mockAdapter{name: "mock", script: `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`}
	wl := &WorkerLoop{config: LoopConfig{Provider: mock, WorkDir: t.TempDir()}}

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
	mock := &mockAdapter{name: "mock", script: `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`}
	wl := &WorkerLoop{config: LoopConfig{Provider: mock, WorkDir: t.TempDir()}}

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
	mock := &mockAdapter{name: "mock", script: `head -c0; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`}
	wl := &WorkerLoop{config: LoopConfig{Provider: mock, WorkDir: t.TempDir()}}

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
