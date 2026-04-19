package worker

import (
	"context"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteSubprocess_HappyPath(t *testing.T) {
	script := `head -c0; echo '{"type":"system.init"}'; echo '{"type":"result","output":"task done","cost_usd":0.03,"duration_ms":500,"session_id":"s1"}'`
	mock := &mockAdapter{name: "mock", script: script}

	wl := &WorkerLoop{config: LoopConfig{Provider: mock}}
	taskCfg := adapter.TaskConfig{TaskID: "test-happy", Prompt: "do work"}

	result, err := wl.executeSubprocess(context.Background(), taskCfg)
	require.NoError(t, err)
	assert.Equal(t, "task done", result.Output)
	assert.InDelta(t, 0.03, result.CostUSD, 0.001)
	assert.Equal(t, int64(500), result.DurationMS)
	assert.Equal(t, "s1", result.SessionID)
}

func TestExecuteSubprocess_SubprocessFailure(t *testing.T) {
	mock := &mockAdapter{name: "mock", script: `head -c0; exit 1`}
	wl := &WorkerLoop{config: LoopConfig{Provider: mock}}

	_, err := wl.executeSubprocess(context.Background(), adapter.TaskConfig{
		TaskID: "test-fail",
		Prompt: "this will fail",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no result event")
}

func TestExecuteSubprocess_ContextCancellation(t *testing.T) {
	mock := &mockAdapter{name: "mock", script: `head -c0; sleep 30`}
	wl := &WorkerLoop{config: LoopConfig{Provider: mock}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := wl.executeSubprocess(ctx, adapter.TaskConfig{
		TaskID: "test-cancel",
		Prompt: "cancel me",
	})
	require.Error(t, err)
}

func TestExecuteSubprocess_FailWithOutput(t *testing.T) {
	script := `head -c0; echo '{"type":"result","output":"partial result","cost_usd":0.01}'; exit 1`
	mock := &mockAdapter{name: "mock", script: script}
	wl := &WorkerLoop{config: LoopConfig{Provider: mock}}

	result, err := wl.executeSubprocess(context.Background(), adapter.TaskConfig{
		TaskID: "test-fail-output",
		Prompt: "fail with output",
	})
	require.NoError(t, err)
	assert.Equal(t, "partial result", result.Output)
}

func TestExecuteSubprocess_NoResultEvent(t *testing.T) {
	script := `head -c0; echo '{"type":"system.init"}'; echo '{"type":"system.task_started"}'`
	mock := &mockAdapter{name: "mock", script: script}
	wl := &WorkerLoop{config: LoopConfig{Provider: mock}}

	_, err := wl.executeSubprocess(context.Background(), adapter.TaskConfig{
		TaskID: "test-no-result",
		Prompt: "no result",
	})
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

	wl := &WorkerLoop{config: LoopConfig{Provider: mock}}
	result, err := wl.executeSubprocess(context.Background(), adapter.TaskConfig{
		TaskID: "test-codex-v2",
		Prompt: "do work",
	})
	require.NoError(t, err)
	assert.Equal(t, "task done via codex", result.Output)
}
