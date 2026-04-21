package worker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionContext_UsesPolicyFileModTimeDeadline(t *testing.T) {
	wl := NewWorkerLoop(LoopConfig{Provider: adapter.NewClaudeAdapter()})
	cache := security.NewPolicyCache()
	taskID := "expired-deadline-task"
	require.NoError(t, cache.Write(taskID, security.SecurityPolicy{TimeoutSec: 1}))
	t.Cleanup(func() { cache.Delete(taskID) })

	expiredAt := time.Now().Add(-2 * time.Second)
	require.NoError(t, os.Chtimes(cache.PolicyPath(taskID), expiredAt, expiredAt))

	ctx, cancel := wl.executionContext(context.Background(), taskID)
	defer cancel()

	<-ctx.Done()
	require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
}

func TestExecuteWithParallel_DoesNotStartExpiredQueuedTask(t *testing.T) {
	mock := &mockAdapter{name: "mock", script: `head -c0; echo '{"type":"result","output":"should not run"}'`}
	wl := NewWorkerLoop(LoopConfig{
		Provider:       mock,
		WorkDir:        t.TempDir(),
		MaxConcurrency: 1,
	})
	wl.configureExecutionConcurrency()

	cache := security.NewPolicyCache()
	taskID := "expired-queued-task"
	require.NoError(t, cache.Write(taskID, security.SecurityPolicy{TimeoutSec: 1}))
	t.Cleanup(func() { cache.Delete(taskID) })

	expiredAt := time.Now().Add(-2 * time.Second)
	require.NoError(t, os.Chtimes(cache.PolicyPath(taskID), expiredAt, expiredAt))

	result, err := wl.executeWithParallel(context.Background(), adapter.TaskConfig{
		TaskID:  taskID,
		Prompt:  "do work",
		WorkDir: t.TempDir(),
	}, nil, newTaskRunMeta(taskID, ""))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "acquire semaphore")
	assert.Empty(t, result.Output)
	assert.Len(t, mock.calls, 0)
}

func TestParseStreamWithBudget_AllowsLargeResultLine(t *testing.T) {
	largeOutput := strings.Repeat("a", 128*1024)
	payload := fmt.Sprintf("{\"type\":\"result\",\"output\":\"%s\"}\n", largeOutput)
	mock := &mockAdapter{name: "mock"}
	wl := &WorkerLoop{
		config: LoopConfig{Provider: mock},
	}

	result, err := wl.parseStream(strings.NewReader(payload), "large-line-task")

	require.NoError(t, err)
	assert.Len(t, result.Output, len(largeOutput))
}
