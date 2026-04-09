// Package pidlock_test tests PID lock acquisition, release, and stale lock reclaim.
package pidlock_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/pidlock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAcquire_NewLock_Success verifies that a new PID lock is acquired when no lock file exists.
func TestAcquire_NewLock_Success(t *testing.T) {
	t.Parallel()

	// Given: a temporary directory with no existing PID file
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "worker.pid")
	lock := pidlock.New(lockPath)

	// When: acquiring the lock
	err := lock.Acquire()

	// Then: no error is returned and the PID file exists with a valid PID
	require.NoError(t, err)
	defer lock.Release() //nolint:errcheck

	pid, err := lock.ReadPID()
	require.NoError(t, err)
	assert.Greater(t, pid, 0, "PID file must contain a positive PID")

	_, statErr := os.Stat(lockPath)
	assert.NoError(t, statErr, "PID file must exist after acquire")
}

// TestAcquire_AlreadyHeld_Error verifies that acquiring a lock held by a running process returns an error.
func TestAcquire_AlreadyHeld_Error(t *testing.T) {
	t.Parallel()

	// Given: a PID file holding the current (running) process PID
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "worker.pid")
	currentPID := os.Getpid()
	err := os.WriteFile(lockPath, []byte(itoa(currentPID)), 0o600)
	require.NoError(t, err)

	lock := pidlock.New(lockPath)

	// When: attempting to acquire the already-held lock
	err = lock.Acquire()

	// Then: an error is returned containing "Worker already running"
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Worker already running")
	assert.Contains(t, err.Error(), itoa(currentPID))
}

// TestAcquire_StaleLock_Reclaim verifies that a stale PID lock (dead process) is reclaimed.
func TestAcquire_StaleLock_Reclaim(t *testing.T) {
	t.Parallel()

	// Given: a PID file with a PID that no longer exists
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "worker.pid")
	// PID 99999999 is virtually guaranteed not to exist
	err := os.WriteFile(lockPath, []byte("99999999"), 0o600)
	require.NoError(t, err)

	lock := pidlock.New(lockPath)

	// When: acquiring the lock with a stale PID file
	err = lock.Acquire()

	// Then: lock is reclaimed without error and PID file is updated to current PID
	require.NoError(t, err)
	defer lock.Release() //nolint:errcheck

	pid, err := lock.ReadPID()
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), pid, "PID file must be updated to current process PID after reclaim")
}

// TestAcquire_StaleUnreadableLock verifies that a lock file with invalid content is treated as stale.
func TestAcquire_StaleUnreadableLock(t *testing.T) {
	t.Parallel()

	// Given: a lock file containing garbage (unreadable PID)
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "worker.pid")
	err := os.WriteFile(lockPath, []byte("garbage"), 0o600)
	require.NoError(t, err)

	lock := pidlock.New(lockPath)

	// When: acquiring the lock
	err = lock.Acquire()

	// Then: garbage content is treated as stale — lock is reclaimed without error
	require.NoError(t, err)
	defer lock.Release() //nolint:errcheck

	pid, err := lock.ReadPID()
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), pid, "PID file must be overwritten with current PID")
}

// TestAcquire_PathNonWritable verifies that Acquire returns an error on a non-writable path.
func TestAcquire_PathNonWritable(t *testing.T) {
	t.Parallel()

	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks, skipping")
	}

	// Given: a lock path inside a read-only directory
	dir := t.TempDir()
	err := os.Chmod(dir, 0o555)
	require.NoError(t, err)
	defer os.Chmod(dir, 0o755) //nolint:errcheck

	lockPath := filepath.Join(dir, "worker.pid")
	lock := pidlock.New(lockPath)

	// When: acquiring on a non-writable directory
	err = lock.Acquire()

	// Then: an error is returned
	require.Error(t, err)
}

// TestAcquire_Release_Cycle verifies that a lock can be acquired, released, and re-acquired.
func TestAcquire_Release_Cycle(t *testing.T) {
	t.Parallel()

	// Given: a lock path
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "worker.pid")
	lock := pidlock.New(lockPath)

	// When: acquire → release → acquire again
	err := lock.Acquire()
	require.NoError(t, err)

	err = lock.Release()
	require.NoError(t, err)

	err = lock.Acquire()
	require.NoError(t, err)
	defer lock.Release() //nolint:errcheck

	// Then: second acquisition succeeds and PID file reflects current PID
	pid, err := lock.ReadPID()
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), pid)
}

// TestAcquire_CreatesMissingDirectory verifies that Acquire creates the parent directory if absent.
func TestAcquire_CreatesMissingDirectory(t *testing.T) {
	t.Parallel()

	// Given: a path whose parent directory does not exist
	base := t.TempDir()
	lockPath := filepath.Join(base, "sub", "worker.pid")
	lock := pidlock.New(lockPath)

	// When: acquiring the lock
	err := lock.Acquire()

	// Then: the directory is created and the lock file exists
	require.NoError(t, err)
	defer lock.Release() //nolint:errcheck

	_, statErr := os.Stat(lockPath)
	assert.NoError(t, statErr, "PID file must exist after Acquire created the parent directory")
}

// TestAcquire_MkdirFailure verifies that Acquire returns an error when the parent directory cannot be created.
func TestAcquire_MkdirFailure(t *testing.T) {
	t.Parallel()

	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks, skipping")
	}

	// Given: a read-only parent directory so MkdirAll cannot create a subdirectory
	base := t.TempDir()
	err := os.Chmod(base, 0o555)
	require.NoError(t, err)
	defer os.Chmod(base, 0o755) //nolint:errcheck

	lockPath := filepath.Join(base, "sub", "worker.pid")
	lock := pidlock.New(lockPath)

	// When: acquiring with an uncreateable parent directory
	err = lock.Acquire()

	// Then: an error is returned mentioning mkdir
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pidlock: mkdir")
}

// itoa converts an int to its string representation.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
