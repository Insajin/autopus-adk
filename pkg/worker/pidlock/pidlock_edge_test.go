// Package pidlock_test tests edge cases for PID lock: missing files, invalid content, release failures.
package pidlock_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/pidlock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRelease_DeletesFile verifies that releasing the lock deletes the PID file.
func TestRelease_DeletesFile(t *testing.T) {
	t.Parallel()

	// Given: an acquired PID lock
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "worker.pid")
	lock := pidlock.New(lockPath)
	err := lock.Acquire()
	require.NoError(t, err)

	_, statErr := os.Stat(lockPath)
	require.NoError(t, statErr, "PID file must exist before release")

	// When: releasing the lock
	err = lock.Release()

	// Then: no error and PID file is deleted
	require.NoError(t, err)
	_, statErr = os.Stat(lockPath)
	assert.True(t, os.IsNotExist(statErr), "PID file must be deleted after release")
}

// TestRelease_NoFileNoError verifies that releasing an un-acquired lock returns no error.
func TestRelease_NoFileNoError(t *testing.T) {
	t.Parallel()

	// Given: a lock that was never acquired (no PID file)
	dir := t.TempDir()
	lock := pidlock.New(filepath.Join(dir, "worker.pid"))

	// When: releasing without prior acquisition
	err := lock.Release()

	// Then: no error (file-not-exist is silently ignored)
	assert.NoError(t, err)
}

// TestRelease_RemoveFailure verifies that Release returns an error when the PID file cannot be removed.
func TestRelease_RemoveFailure(t *testing.T) {
	t.Parallel()

	if os.Getuid() == 0 {
		t.Skip("root can remove read-only files, skipping")
	}

	// Given: a directory containing a PID file, then made read-only so remove fails
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "worker.pid")
	err := os.WriteFile(lockPath, []byte("1"), 0o600)
	require.NoError(t, err)

	// Make the directory read-only to prevent removal of the PID file.
	err = os.Chmod(dir, 0o555)
	require.NoError(t, err)
	defer os.Chmod(dir, 0o755) //nolint:errcheck

	lock := pidlock.New(lockPath)

	// When: releasing (no file handle held, so flock path skipped)
	err = lock.Release()

	// Then: an error is returned because Remove fails
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pidlock: remove")
}

// TestReadPID_ValidFile verifies that ReadPID correctly reads the PID from the lock file.
func TestReadPID_ValidFile(t *testing.T) {
	t.Parallel()

	// Given: a PID file containing a known PID value
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "worker.pid")
	expectedPID := 42
	err := os.WriteFile(lockPath, []byte("42"), 0o600)
	require.NoError(t, err)

	lock := pidlock.New(lockPath)

	// When: reading the PID from the file
	pid, err := lock.ReadPID()

	// Then: the returned PID matches the written value
	require.NoError(t, err)
	assert.Equal(t, expectedPID, pid)
}

// TestReadPID_InvalidContent verifies that ReadPID returns an error for non-integer content.
func TestReadPID_InvalidContent(t *testing.T) {
	t.Parallel()

	// Given: a PID file containing non-numeric content
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "worker.pid")
	err := os.WriteFile(lockPath, []byte("not-a-pid"), 0o600)
	require.NoError(t, err)

	lock := pidlock.New(lockPath)

	// When: reading the PID
	_, err = lock.ReadPID()

	// Then: an error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse PID")
}

// TestReadPID_EmptyFile verifies that ReadPID returns an error for an empty file.
func TestReadPID_EmptyFile(t *testing.T) {
	t.Parallel()

	// Given: an empty PID file
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "worker.pid")
	err := os.WriteFile(lockPath, []byte(""), 0o600)
	require.NoError(t, err)

	lock := pidlock.New(lockPath)

	// When: reading the PID
	_, err = lock.ReadPID()

	// Then: an error is returned
	require.Error(t, err)
}

// TestReadPID_MissingFile verifies that ReadPID returns an error when the file does not exist.
func TestReadPID_MissingFile(t *testing.T) {
	t.Parallel()

	// Given: a lock pointing to a non-existent file
	lock := pidlock.New("/tmp/nonexistent-autopus-worker-xyzzy.pid")

	// When: reading the PID
	_, err := lock.ReadPID()

	// Then: an error is returned mentioning the read failure
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pidlock: read")
}

// TestDefaultPath verifies DefaultPath returns a path ending in worker.pid.
func TestDefaultPath(t *testing.T) {
	t.Parallel()

	// When: calling DefaultPath
	p := pidlock.DefaultPath()

	// Then: the path ends with .autopus/worker.pid
	assert.True(t, len(p) > 0, "DefaultPath must return a non-empty path")
	assert.Equal(t, "worker.pid", filepath.Base(p), "DefaultPath base name must be worker.pid")
	assert.Equal(t, ".autopus", filepath.Base(filepath.Dir(p)), "DefaultPath parent dir must be .autopus")
}

// TestDefaultPath_HomeFallback verifies DefaultPath uses a relative fallback when HOME is unset.
func TestDefaultPath_HomeFallback(t *testing.T) {
	// Not parallel — modifies HOME environment variable.
	t.Setenv("HOME", "")

	// When: HOME is empty and UserHomeDir fails
	p := pidlock.DefaultPath()

	// Then: fallback path is returned with correct base name
	assert.Equal(t, "worker.pid", filepath.Base(p), "fallback DefaultPath base must be worker.pid")
	assert.Equal(t, ".autopus", filepath.Base(filepath.Dir(p)), "fallback parent dir must be .autopus")
}
