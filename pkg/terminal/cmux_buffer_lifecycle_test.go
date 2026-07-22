package terminal

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCmuxAdapter_SendLongText_ReusesSingleClearedBuffer(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	adapter := &CmuxAdapter{}
	require.NoError(t, adapter.SendLongText(context.Background(), "surface:7", "first"))
	require.NoError(t, adapter.SendLongText(context.Background(), "surface:8", "second"))

	require.Len(t, captured.calls, 6)
	for _, call := range captured.calls {
		assert.NotEqual(t, "delete-buffer", call.args[0])
	}
	for _, offset := range []int{0, 3} {
		assert.Equal(t, []string{
			"set-buffer", "--name", "autopus-input", "--", captured.calls[offset].args[4],
		}, captured.calls[offset].args)
		assert.Equal(t, "paste-buffer", captured.calls[offset+1].args[0])
		assert.Equal(t, []string{
			"set-buffer", "--name", "autopus-input", "--", "",
		}, captured.calls[offset+2].args)
	}
}

func TestCmuxAdapter_SendLongText_ConcurrentCallsAreSerialized(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	original := execCommand
	originalContext := execCommandContext
	firstSetEntered := make(chan struct{})
	releaseFirstSet := make(chan struct{})
	secondSetEntered := make(chan struct{})
	var payloadSets atomic.Int32

	buildCommand := func(_ string, args ...string) *exec.Cmd {
		if len(args) == 5 && args[0] == "set-buffer" && args[4] != "" {
			switch payloadSets.Add(1) {
			case 1:
				close(firstSetEntered)
				<-releaseFirstSet
			case 2:
				close(secondSetEntered)
			}
		}
		return exec.Command("true")
	}
	execCommand = buildCommand
	execCommandContext = func(_ context.Context, name string, args ...string) *exec.Cmd {
		return buildCommand(name, args...)
	}
	t.Cleanup(func() {
		execCommand = original
		execCommandContext = originalContext
	})

	adapter := &CmuxAdapter{}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for pane, payload := range map[PaneID]string{
		"surface:7": "first",
		"surface:8": strings.Repeat("second", 10),
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- adapter.SendLongText(context.Background(), pane, payload)
		}()
		if pane == "surface:7" {
			<-firstSetEntered
		}
	}

	overlapped := false
	select {
	case <-secondSetEntered:
		overlapped = true
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirstSet)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	assert.False(t, overlapped, "set-buffer/paste-buffer transactions must not overlap")
}

func TestCmuxAdapter_SendLongText_ClearFailureDoesNotFailDeliveredInput(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	var logs bytes.Buffer
	originalLogWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(originalLogWriter) })
	original := execCommand
	originalContext := execCommandContext
	var calls [][]string
	buildCommand := func(_ string, args ...string) *exec.Cmd {
		calls = append(calls, args)
		if len(args) == 5 && args[0] == "set-buffer" && args[4] == "" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	execCommand = buildCommand
	execCommandContext = func(_ context.Context, name string, args ...string) *exec.Cmd {
		return buildCommand(name, args...)
	}
	t.Cleanup(func() {
		execCommand = original
		execCommandContext = originalContext
	})

	adapter := &CmuxAdapter{}
	err := adapter.SendLongText(context.Background(), "surface:7", "delivered")
	nextErr := adapter.SendLongText(context.Background(), "surface:7", "next")

	require.NoError(t, err)
	require.NoError(t, nextErr)
	require.Len(t, calls, 6)
	assert.Equal(t, "", calls[2][4])
	assert.Equal(t, "next", calls[3][4], "the next transaction must overwrite stale buffer content")
	assert.Contains(t, logs.String(), "cmux: clear input buffer")
	for _, call := range calls {
		assert.NotEqual(t, "send", call[0], "clear failure must not resend delivered input")
	}
}

func TestCmuxAdapter_SendLongText_ActiveCrossProcessLockHonorsCancellation(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	lockPath := filepath.Join(t.TempDir(), "cmux-buffer.lock")
	restoreLockConfig := setCmuxBufferLockConfigForTest(lockPath, time.Second)
	t.Cleanup(restoreLockConfig)
	release, err := acquireCmuxInputBuffer(context.Background())
	require.NoError(t, err)
	t.Cleanup(release)
	restoreExec, captured := newCmuxMockV2("", nil)
	defer restoreExec()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err = (&CmuxAdapter{}).SendLongText(ctx, "surface:7", "prompt")

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Empty(t, captured.calls)
}

func TestCmuxAdapter_SendLongText_LiveContentionDoesNotFallbackTransport(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	lockPath := filepath.Join(t.TempDir(), "cmux-buffer.lock")
	restoreLockConfig := setCmuxBufferLockConfigForTest(lockPath, 30*time.Millisecond)
	t.Cleanup(restoreLockConfig)
	release, err := acquireCmuxInputBuffer(context.Background())
	require.NoError(t, err)
	t.Cleanup(release)
	restoreExec, captured := newCmuxMockV2("", nil)
	defer restoreExec()

	err = (&CmuxAdapter{}).SendLongText(context.Background(), "surface:7", "fallback")

	require.Error(t, err)
	assert.ErrorIs(t, err, errCmuxBufferLockBusy)
	assert.Empty(t, captured.calls)
}

func TestCmuxAdapter_SendLongText_BufferFailureReleasesLockBeforeFallback(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	for _, failCommand := range []string{"set-buffer", "paste-buffer"} {
		t.Run(failCommand, func(t *testing.T) {
			lockPath := filepath.Join(t.TempDir(), "cmux-buffer.lock")
			restoreLockConfig := setCmuxBufferLockConfigForTest(lockPath, time.Second)
			t.Cleanup(restoreLockConfig)
			original := execCommand
			originalContext := execCommandContext
			lockAvailableAtFallback := false
			buildCommand := func(_ string, args ...string) *exec.Cmd {
				if args[0] == "send" {
					probeRelease, lockErr := acquireCmuxInputBuffer(context.Background())
					if lockErr == nil {
						lockAvailableAtFallback = true
						probeRelease()
					}
				}
				if args[0] == failCommand && !(args[0] == "set-buffer" && args[len(args)-1] == "") {
					return exec.Command("false")
				}
				return exec.Command("true")
			}
			execCommand = buildCommand
			execCommandContext = func(_ context.Context, name string, args ...string) *exec.Cmd {
				return buildCommand(name, args...)
			}
			t.Cleanup(func() {
				execCommand = original
				execCommandContext = originalContext
			})

			err := (&CmuxAdapter{}).SendLongText(context.Background(), "surface:7", "fallback")

			require.NoError(t, err)
			assert.True(t, lockAvailableAtFallback)
		})
	}
}

func TestAcquireCmuxInputBuffer_ReleaseDoesNotRemoveReplacementLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "cmux-buffer.lock")
	restoreLockConfig := setCmuxBufferLockConfigForTest(lockPath, time.Second)
	t.Cleanup(restoreLockConfig)
	release, err := acquireCmuxInputBuffer(context.Background())
	require.NoError(t, err)
	require.NoError(t, os.Remove(lockPath))
	require.NoError(t, os.WriteFile(lockPath, []byte("replacement"), 0o600))

	release()

	info, err := os.Stat(lockPath)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())
}

func setCmuxBufferLockConfigForTest(
	path string,
	waitLimit time.Duration,
) func() {
	originalPath := cmuxBufferLockPath
	originalWaitLimit := cmuxBufferLockWaitLimit
	cmuxBufferLockPath = path
	cmuxBufferLockWaitLimit = waitLimit
	return func() {
		cmuxBufferLockPath = originalPath
		cmuxBufferLockWaitLimit = originalWaitLimit
	}
}
