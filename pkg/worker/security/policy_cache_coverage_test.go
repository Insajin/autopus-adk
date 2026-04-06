// Package security — additional PolicyCache coverage tests.
package security

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteWithLstatGuard_DirIsSymlink verifies that write is rejected when
// the cache directory itself is a symlink.
func TestWriteWithLstatGuard_DirIsSymlink(t *testing.T) {
	t.Parallel()

	// Given: a real directory and a symlink pointing to it as the cache dir
	realDir := t.TempDir()
	symlinkDir := filepath.Join(t.TempDir(), "symlink-cache")
	require.NoError(t, os.Symlink(realDir, symlinkDir))

	cache := &PolicyCache{dir: symlinkDir}

	// When: we attempt to write
	err := cache.WriteWithLstatGuard("task-dir-symlink", SecurityPolicy{})

	// Then: write is rejected because the dir itself is a symlink
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")
}

// TestWriteWithLstatGuard_MkdirAllFails verifies error when dir cannot be created.
func TestWriteWithLstatGuard_MkdirAllFails(t *testing.T) {
	t.Parallel()

	// Use a file as the parent to block directory creation.
	tmpFile := filepath.Join(t.TempDir(), "blockfile")
	require.NoError(t, os.WriteFile(tmpFile, []byte("x"), 0600))

	cache := &PolicyCache{dir: filepath.Join(tmpFile, "subdir")}

	err := cache.WriteWithLstatGuard("task-mkdir-fail", SecurityPolicy{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create policy dir")
}

// TestWriteWithLstatGuard_RenameToSymlinkDir verifies that rename to a cross-device
// path fails gracefully.
func TestWriteWithLstatGuard_NormalWriteSucceeds(t *testing.T) {
	t.Parallel()

	// Given: a normal (non-symlink) directory
	cacheDir := t.TempDir()
	cache := &PolicyCache{dir: cacheDir}
	policy := SecurityPolicy{AllowedCommands: []string{"go "}}

	// When: write with Lstat guard
	err := cache.WriteWithLstatGuard("normal-task-lstat", policy)
	require.NoError(t, err)

	// Then: policy can be read back
	got, err := cache.Read("normal-task-lstat")
	require.NoError(t, err)
	assert.Equal(t, policy.AllowedCommands, got.AllowedCommands)
}

// TestWriteWithLstatGuard_OverwriteExisting verifies overwrite of existing policy.
func TestWriteWithLstatGuard_OverwriteExisting(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	cache := &PolicyCache{dir: cacheDir}

	// Write initial policy.
	p1 := SecurityPolicy{AllowedCommands: []string{"git "}}
	require.NoError(t, cache.WriteWithLstatGuard("overwrite-task", p1))

	// Overwrite with new policy.
	p2 := SecurityPolicy{AllowedCommands: []string{"go "}, TimeoutSec: 30}
	require.NoError(t, cache.WriteWithLstatGuard("overwrite-task", p2))

	// Read back — should be p2.
	got, err := cache.Read("overwrite-task")
	require.NoError(t, err)
	assert.Equal(t, []string{"go "}, got.AllowedCommands)
	assert.Equal(t, 30, got.TimeoutSec)
}

// TestCheckSymlink_NonExistentPath verifies that a non-existent path is safe (no error).
func TestCheckSymlink_NonExistentPath(t *testing.T) {
	t.Parallel()

	err := checkSymlink("/tmp/this-path-does-not-exist-autopus-test-12345")
	assert.NoError(t, err, "non-existent path should not trigger symlink error")
}

// TestCheckSymlink_RegularFile verifies that a regular file is safe.
func TestCheckSymlink_RegularFile(t *testing.T) {
	t.Parallel()

	f := filepath.Join(t.TempDir(), "regular.json")
	require.NoError(t, os.WriteFile(f, []byte("{}"), 0600))

	assert.NoError(t, checkSymlink(f))
}

// TestCheckSymlink_Symlink verifies that a symlink is detected.
func TestCheckSymlink_Symlink(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "target.json")
	require.NoError(t, os.WriteFile(target, []byte("{}"), 0600))

	link := filepath.Join(t.TempDir(), "link.json")
	require.NoError(t, os.Symlink(target, link))

	err := checkSymlink(link)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")
}
