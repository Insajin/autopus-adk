package selfupdate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReplace_Success verifies that an existing binary is atomically replaced
// with a new binary, preserving the original file permissions.
func TestReplace_Success(t *testing.T) {
	t.Parallel()

	destDir := t.TempDir()
	targetPath := filepath.Join(destDir, "autopus-adk")
	originalContent := []byte("original binary content")
	require.NoError(t, os.WriteFile(targetPath, originalContent, 0755))

	newBinaryPath := filepath.Join(t.TempDir(), "autopus-adk-new")
	newContent := []byte("new binary content v0.7.0")
	require.NoError(t, os.WriteFile(newBinaryPath, newContent, 0644))

	r := NewReplacer()
	err := r.Replace(newBinaryPath, targetPath)

	require.NoError(t, err)

	gotContent, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.Equal(t, newContent, gotContent)

	info, err := os.Stat(targetPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
}

// TestReplace_TargetNotFound verifies that replacing a non-existent target
// returns an error from os.Stat.
func TestReplace_TargetNotFound(t *testing.T) {
	t.Parallel()

	newBinaryPath := filepath.Join(t.TempDir(), "autopus-adk-new")
	require.NoError(t, os.WriteFile(newBinaryPath, []byte("new content"), 0755))
	nonExistentTarget := filepath.Join(t.TempDir(), "does-not-exist")

	r := NewReplacer()
	err := r.Replace(newBinaryPath, nonExistentTarget)

	require.Error(t, err)
}

// TestReplace_PermissionError verifies that attempting to replace a binary in
// a read-only directory returns an error with actionable guidance.
func TestReplace_PermissionError(t *testing.T) {
	t.Parallel()

	readOnlyDir := t.TempDir()
	targetPath := filepath.Join(readOnlyDir, "autopus-adk")
	require.NoError(t, os.WriteFile(targetPath, []byte("original"), 0755))
	require.NoError(t, os.Chmod(readOnlyDir, 0555))
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0755) })

	newBinaryPath := filepath.Join(t.TempDir(), "autopus-adk-new")
	require.NoError(t, os.WriteFile(newBinaryPath, []byte("new content"), 0755))

	r := NewReplacer()
	err := r.Replace(newBinaryPath, targetPath)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission")
}

// TestReplace_RestoreOnFailure verifies that if the new binary copy/rename
// fails, the old binary is restored from .old.
func TestReplace_RestoreOnFailure(t *testing.T) {
	t.Parallel()

	r := NewReplacer()

	destDir := t.TempDir()
	targetPath := filepath.Join(destDir, "auto.exe")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0755))

	// Use a non-existent new binary path to trigger rename/copy failure.
	err := r.Replace("/nonexistent/path/auto.exe", targetPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "새 바이너리 교체 실패")

	// Old binary should be restored.
	got, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("old"), got)
}
