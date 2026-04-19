package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/budget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleTask_EnforcesServerIssuedIterationBudget(t *testing.T) {
	mock := &mockAdapter{name: "mock", script: `head -c0; echo '{"type":"tool_call"}'; echo '{"type":"result","output":"done","cost_usd":0.02,"duration_ms":300}'`}
	wl := &WorkerLoop{config: LoopConfig{Provider: mock, WorkDir: t.TempDir()}}

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
	wl := &WorkerLoop{config: LoopConfig{Provider: mock, WorkDir: t.TempDir()}}

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
	wl := &WorkerLoop{config: LoopConfig{Provider: mock, WorkDir: t.TempDir()}}

	payload, _ := json.Marshal(taskPayloadMessage{
		Description:    "test task",
		PipelinePhases: []string{"planner", "deployer"},
	})

	_, err := wl.handleTask(context.Background(), "task-invalid-phase", payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported phase")
}
