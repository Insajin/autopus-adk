package terminal

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	cmuxLockChildModeEnv  = "AUTOPUS_TEST_CMUX_LOCK_CHILD"
	cmuxLockChildPathEnv  = "AUTOPUS_TEST_CMUX_LOCK_PATH"
	cmuxLockChildReadyEnv = "AUTOPUS_TEST_CMUX_LOCK_READY"
)

func TestCmuxBufferLock_LiveProcessContentionDoesNotFallbackTransport(t *testing.T) {
	if runCmuxBufferLockChild() {
		return
	}
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	tempDir := t.TempDir()
	lockPath := filepath.Join(tempDir, "buffer.lock")
	readyPath := filepath.Join(tempDir, "ready")
	cmd := exec.Command(os.Args[0], "-test.run=^TestCmuxBufferLock_LiveProcessContentionDoesNotFallbackTransport$")
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	cmd.Env = append(os.Environ(),
		cmuxLockChildModeEnv+"=hold",
		cmuxLockChildPathEnv+"="+lockPath,
		cmuxLockChildReadyEnv+"="+readyPath,
	)
	require.NoError(t, cmd.Start())
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			_ = stdin.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	require.Eventually(t, func() bool {
		_, statErr := os.Stat(readyPath)
		return statErr == nil
	}, time.Second, 10*time.Millisecond)

	restoreConfig := setCmuxBufferLockConfigForTest(lockPath, 40*time.Millisecond)
	t.Cleanup(restoreConfig)
	restoreExec, captured := newCmuxMockV2("", nil)
	defer restoreExec()
	err = (&CmuxAdapter{}).SendLongText(context.Background(), "surface:7", "must-not-bypass")

	require.Error(t, err)
	assert.ErrorIs(t, err, errCmuxBufferLockBusy)
	assert.Empty(t, captured.calls)
	require.NoError(t, stdin.Close())
	require.NoError(t, cmd.Wait())
	stopped = true
}

func TestCmuxBufferLock_CrashedOwnerIsRecoveredImmediately(t *testing.T) {
	if runCmuxBufferLockChild() {
		return
	}
	lockPath := filepath.Join(t.TempDir(), "buffer.lock")
	cmd := exec.Command(os.Args[0], "-test.run=^TestCmuxBufferLock_CrashedOwnerIsRecoveredImmediately$")
	cmd.Env = append(os.Environ(),
		cmuxLockChildModeEnv+"=crash",
		cmuxLockChildPathEnv+"="+lockPath,
	)
	require.NoError(t, cmd.Run())
	restoreConfig := setCmuxBufferLockConfigForTest(lockPath, 200*time.Millisecond)
	t.Cleanup(restoreConfig)

	started := time.Now()
	release, err := acquireCmuxInputBuffer(context.Background())
	require.NoError(t, err)
	release()
	assert.Less(t, time.Since(started), 100*time.Millisecond)
}

func TestCmuxBufferLock_UnsafePathFailsClosed(t *testing.T) {
	for _, setup := range []struct {
		name string
		run  func(*testing.T, string)
	}{
		{
			name: "symlink",
			run: func(t *testing.T, path string) {
				target := filepath.Join(t.TempDir(), "target")
				require.NoError(t, os.WriteFile(target, nil, 0o600))
				require.NoError(t, os.Symlink(target, path))
			},
		},
		{
			name: "permissive mode",
			run: func(t *testing.T, path string) {
				require.NoError(t, os.WriteFile(path, nil, 0o600))
				require.NoError(t, os.Chmod(path, 0o644))
			},
		},
	} {
		t.Run(setup.name, func(t *testing.T) {
			lockPath := filepath.Join(t.TempDir(), "buffer.lock")
			setup.run(t, lockPath)
			restoreConfig := setCmuxBufferLockConfigForTest(lockPath, 50*time.Millisecond)
			t.Cleanup(restoreConfig)

			release, err := acquireCmuxInputBuffer(context.Background())

			require.Error(t, err)
			assert.Nil(t, release)
		})
	}
}

func runCmuxBufferLockChild() bool {
	mode := os.Getenv(cmuxLockChildModeEnv)
	if mode == "" {
		return false
	}
	cmuxBufferLockPath = os.Getenv(cmuxLockChildPathEnv)
	cmuxBufferLockWaitLimit = time.Second
	release, err := acquireCmuxInputBuffer(context.Background())
	if err != nil {
		os.Exit(3)
	}
	if mode == "crash" {
		os.Exit(0)
	}
	if err := os.WriteFile(os.Getenv(cmuxLockChildReadyEnv), []byte("ready"), 0o600); err != nil {
		os.Exit(4)
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	release()
	return true
}
