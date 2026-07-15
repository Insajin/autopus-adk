package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStreamWithTaskConfig_LateUsageAndFinalOutputAreBothPreserved(t *testing.T) {
	input, output := int64(100), int64(30)
	usage := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "adapter-run", CallID: "adapter-call", Source: telemetry.UsageSourceProvider,
		InputTokensTotal: &input, OutputTokensTotal: &output,
	})
	usage.RunID, usage.CallID = "", ""
	mock := &mockAdapter{
		name:   "mock",
		script: "true",
		parseFn: func(line []byte) (adapter.StreamEvent, error) {
			if strings.Contains(string(line), "usage") {
				return adapter.StreamEvent{Type: "usage", Usage: []telemetry.UsageEnvelope{usage}}, nil
			}
			return adapter.StreamEvent{Type: "result", Data: []byte(`{"output":"done"}`)}, nil
		},
		extractFn: func(event adapter.StreamEvent) adapter.TaskResult {
			return adapter.TaskResult{Output: "done", Usage: event.Usage}
		},
	}
	wl := &WorkerLoop{config: LoopConfig{Provider: mock}}
	cfg := adapter.TaskConfig{
		TaskID: "task-1", RunID: "run-1", CallID: "call-1", Attempt: 2,
		Model: "model-1", Effort: "high", Phase: "execute", Role: "executor",
	}

	result, err := wl.parseStreamWithTaskConfig(strings.NewReader("usage\nresult\n"), cfg, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "done", result.Output)
	require.Len(t, result.Usage, 1)
	assert.Equal(t, "run-1", result.Usage[0].RunID)
	assert.Equal(t, "call-1", result.Usage[0].CallID)
	assert.Equal(t, 2, result.Usage[0].Attempt)
	assert.Equal(t, int64(130), *result.Usage[0].RawTotalTokens)
}

func TestPipelineAggregateResults_PreservesUsageAndToolCalls(t *testing.T) {
	first := workerUsage("run", "call-1", 100, 20)
	second := workerUsage("run", "call-2", 80, 10)
	pe := NewPipelineExecutor(adapter.NewClaudeAdapter(), "", t.TempDir())

	got := pe.aggregateResults([]PhaseResult{
		{Phase: PhasePlanner, Output: "plan", Usage: []telemetry.UsageEnvelope{first}, ToolCalls: 2},
		{Phase: PhaseExecutor, Output: "done", Usage: []telemetry.UsageEnvelope{second}, ToolCalls: 3},
	}, 0.1, 50)

	assert.Contains(t, got.Output, "plan")
	assert.Contains(t, got.Output, "done")
	assert.Equal(t, 5, got.ToolCalls)
	require.Len(t, got.Usage, 2)
	aggregate := telemetry.AggregateUsage(got.Usage)
	assert.Equal(t, 2, aggregate.UniqueModelCallCount)
	assert.Equal(t, int64(210), *aggregate.RawTotalTokens)
}

func TestHandleTask_UsageRecorderRunsExactlyOnceAtSuccessBoundary(t *testing.T) {
	input, output := int64(90), int64(10)
	unbound := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "adapter", CallID: "adapter", Source: telemetry.UsageSourceProvider,
		InputTokensTotal: &input, OutputTokensTotal: &output,
	})
	unbound.RunID, unbound.CallID = "", ""
	mock := &mockAdapter{
		name: "mock", script: `echo result`,
		parseFn: func([]byte) (adapter.StreamEvent, error) {
			return adapter.StreamEvent{Type: "result", Usage: []telemetry.UsageEnvelope{unbound}}, nil
		},
		extractFn: func(event adapter.StreamEvent) adapter.TaskResult {
			return adapter.TaskResult{Output: "done", Usage: event.Usage, ToolCalls: 2}
		},
	}
	var recorded []telemetry.AgentRun
	wl := &WorkerLoop{config: LoopConfig{
		Provider: mock, WorkDir: t.TempDir(), WorkerName: "executor",
		RecordAgentRun: func(run telemetry.AgentRun) error {
			recorded = append(recorded, run)
			return nil
		},
	}}
	payload, err := json.Marshal(taskPayloadMessage{Description: "test", SpecID: "SPEC-1", Attempt: 2, Effort: "high"})
	require.NoError(t, err)

	_, err = wl.handleTask(context.Background(), "task-1", payload)

	require.NoError(t, err)
	require.Len(t, recorded, 1)
	assert.Equal(t, telemetry.StatusPass, recorded[0].Status)
	assert.Equal(t, telemetry.StatusPass, recorded[0].AcceptanceStatus)
	assert.Equal(t, 2, recorded[0].Attempt)
	require.Len(t, recorded[0].Usage, 1)
	assert.NotEmpty(t, recorded[0].Usage[0].RunID)
	assert.NotEmpty(t, recorded[0].Usage[0].CallID)
}

func TestHandleTask_UsageRecorderRunsExactlyOnceAtFailureBoundary(t *testing.T) {
	input, output := int64(90), int64(10)
	usage := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "adapter", CallID: "adapter", Source: telemetry.UsageSourceProvider,
		InputTokensTotal: &input, OutputTokensTotal: &output,
	})
	usage.RunID, usage.CallID = "", ""
	mock := &mockAdapter{
		name: "mock", script: `echo result`,
		parseFn: func([]byte) (adapter.StreamEvent, error) {
			return adapter.StreamEvent{Type: "result", Usage: []telemetry.UsageEnvelope{usage}}, nil
		},
		extractFn: func(event adapter.StreamEvent) adapter.TaskResult {
			return adapter.TaskResult{IsError: true, Error: "provider failed", Usage: event.Usage}
		},
	}
	var recorded []telemetry.AgentRun
	wl := &WorkerLoop{config: LoopConfig{
		Provider: mock, WorkDir: t.TempDir(),
		RecordAgentRun: func(run telemetry.AgentRun) error {
			recorded = append(recorded, run)
			return nil
		},
	}}
	payload, err := json.Marshal(taskPayloadMessage{Description: "test", SpecID: "SPEC-FAIL"})
	require.NoError(t, err)

	_, err = wl.handleTask(context.Background(), "task-fail", payload)

	require.Error(t, err)
	require.Len(t, recorded, 1)
	assert.Equal(t, telemetry.StatusFail, recorded[0].Status)
	assert.Equal(t, telemetry.StatusFail, recorded[0].AcceptanceStatus)
	require.Len(t, recorded[0].Usage, 1)
	assert.Equal(t, telemetry.UsageStatusActual, recorded[0].Usage[0].UsageStatus)
}

func TestHandleTask_PipelinePropagatesRunAttemptEffortRoleAndDistinctPhaseCalls(t *testing.T) {
	mock := &mockAdapter{name: "mock", script: `echo '{"type":"result","output":"done"}'`}
	wl := &WorkerLoop{config: LoopConfig{Provider: mock, WorkDir: t.TempDir()}}
	payload, err := json.Marshal(taskPayloadMessage{
		Prompt: "pipeline", PipelinePhases: []string{"planner", "reviewer"},
		Attempt: 2, Effort: "high", Role: "deep-worker", Model: "model-1",
	})
	require.NoError(t, err)

	_, err = wl.handleTask(context.Background(), "task-pipeline-usage", payload)

	require.NoError(t, err)
	require.Len(t, mock.calls, 2)
	assert.NotEmpty(t, mock.calls[0].RunID)
	assert.Equal(t, mock.calls[0].RunID, mock.calls[1].RunID)
	assert.NotEqual(t, mock.calls[0].CallID, mock.calls[1].CallID)
	for _, call := range mock.calls {
		assert.Equal(t, 2, call.Attempt)
		assert.Equal(t, "high", call.Effort)
		assert.Equal(t, "deep-worker", call.Role)
		assert.Equal(t, "model-1", call.Model)
	}
}

func TestEnsureUsageIdentity_RetryGetsNewCallIDWithinStableRun(t *testing.T) {
	first := ensureUsageIdentity(adapter.TaskConfig{TaskID: "task", RunID: "run", Attempt: 1}, "execute", "executor")
	retry := ensureUsageIdentity(adapter.TaskConfig{TaskID: "task", RunID: "run", Attempt: 2}, "execute", "executor")

	assert.Equal(t, first.RunID, retry.RunID)
	assert.NotEqual(t, first.CallID, retry.CallID)
	assert.Equal(t, 1, first.Attempt)
	assert.Equal(t, 2, retry.Attempt)
}

func workerUsage(runID, callID string, input, output int64) telemetry.UsageEnvelope {
	return telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: runID, CallID: callID, TaskID: "task", Source: telemetry.UsageSourceProvider,
		InputTokensTotal: &input, OutputTokensTotal: &output,
	})
}
