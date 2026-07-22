package terminal

import (
	"context"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const expectedCmuxBufferSentinel = "[autopus-cleared]"

func TestCmuxAdapter_SendLongText_WaitsForAsyncPasteBeforeSentinelOverwrite(t *testing.T) {
	require.Equal(t, expectedCmuxBufferSentinel, cmuxInputBufferSentinel)
	require.Equal(t, time.Second, cmuxBufferPostPasteMaxDelay)
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	restoreConfig := setCmuxBufferLockConfigForTest(
		filepath.Join(t.TempDir(), "buffer.lock"), time.Second,
	)
	t.Cleanup(restoreConfig)
	original := execCommand
	originalContext := execCommandContext
	var mu sync.Mutex
	bufferContent := ""
	consumed := make(chan string, 1)
	buildCommand := func(_ context.Context, _ string, args ...string) *exec.Cmd {
		switch args[0] {
		case "set-buffer":
			mu.Lock()
			bufferContent = args[len(args)-1]
			mu.Unlock()
		case "paste-buffer":
			go func() {
				time.Sleep(75 * time.Millisecond)
				mu.Lock()
				consumed <- bufferContent
				mu.Unlock()
			}()
		}
		return exec.Command("true")
	}
	execCommand = func(name string, args ...string) *exec.Cmd {
		return buildCommand(context.Background(), name, args...)
	}
	execCommandContext = buildCommand
	t.Cleanup(func() {
		execCommand = original
		execCommandContext = originalContext
	})

	started := time.Now()
	err := (&CmuxAdapter{}).SendLongText(context.Background(), "surface:7", "sensitive prompt")
	elapsed := time.Since(started)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, cmuxBufferPasteSettleDelay)
	assert.Less(t, elapsed, cmuxBufferPostPasteMaxDelay+100*time.Millisecond)
	select {
	case delivered := <-consumed:
		assert.Equal(t, "sensitive prompt", delivered)
	case <-time.After(time.Second):
		t.Fatal("async paste was not consumed")
	}
	mu.Lock()
	retained := bufferContent
	mu.Unlock()
	assert.Equal(t, expectedCmuxBufferSentinel, retained)
	assert.NotContains(t, retained, "sensitive prompt")
}

func TestCmuxAdapter_SendLongText_ClearUsesIndependentContextAfterCallerCancel(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	for _, pasteFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "paste success", true: "paste failure"}[pasteFails], func(t *testing.T) {
			restoreConfig := setCmuxBufferLockConfigForTest(
				filepath.Join(t.TempDir(), "buffer.lock"), time.Second,
			)
			t.Cleanup(restoreConfig)
			original := execCommand
			originalContext := execCommandContext
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var clearContextErr error
			var clearHasDeadline bool
			var clearDeadlineRemaining time.Duration
			sendCalls := 0
			buildCommand := func(commandCtx context.Context, _ string, args ...string) *exec.Cmd {
				if args[0] == "paste-buffer" {
					cancel()
					if pasteFails {
						return exec.Command("false")
					}
				}
				if args[0] == "set-buffer" && args[len(args)-1] == expectedCmuxBufferSentinel {
					clearContextErr = commandCtx.Err()
					deadline, ok := commandCtx.Deadline()
					clearHasDeadline = ok
					clearDeadlineRemaining = time.Until(deadline)
				}
				if args[0] == "send" {
					sendCalls++
				}
				return exec.Command("true")
			}
			execCommand = func(name string, args ...string) *exec.Cmd {
				return buildCommand(context.Background(), name, args...)
			}
			execCommandContext = buildCommand
			t.Cleanup(func() {
				execCommand = original
				execCommandContext = originalContext
			})

			started := time.Now()
			err := (&CmuxAdapter{}).SendLongText(ctx, "surface:7", "prompt")

			assert.ErrorIs(t, err, context.Canceled)
			assert.NoError(t, clearContextErr)
			assert.True(t, clearHasDeadline)
			assert.Positive(t, clearDeadlineRemaining)
			assert.LessOrEqual(t, clearDeadlineRemaining, time.Second)
			assert.Less(t, time.Since(started), cmuxBufferPostPasteMaxDelay+100*time.Millisecond)
			assert.Zero(t, sendCalls)
		})
	}
}
