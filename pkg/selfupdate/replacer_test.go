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
// R4: atomic replace via os.Rename. R5: preserve file permissions.
func TestReplace_Success(t *testing.T) {
	t.Parallel()

	// Given: an existing binary with specific content and permissions
	destDir := t.TempDir()
	targetPath := filepath.Join(destDir, "autopus-adk")
	originalContent := []byte("original binary content")
	require.NoError(t, os.WriteFile(targetPath, originalContent, 0755))

	newBinaryPath := filepath.Join(t.TempDir(), "autopus-adk-new")
	newContent := []byte("new binary content v0.7.0")
	require.NoError(t, os.WriteFile(newBinaryPath, newContent, 0644))

	// When: Replace is called
	r := NewReplacer()
	err := r.Replace(newBinaryPath, targetPath)

	// Then: target contains new content and permissions are preserved
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

	// Given: a new binary and a target path that does not exist
	newBinaryPath := filepath.Join(t.TempDir(), "autopus-adk-new")
	require.NoError(t, os.WriteFile(newBinaryPath, []byte("new content"), 0755))
	nonExistentTarget := filepath.Join(t.TempDir(), "does-not-exist")

	// When: Replace is called with a non-existent target
	r := NewReplacer()
	err := r.Replace(newBinaryPath, nonExistentTarget)

	// Then: error is returned
	require.Error(t, err)
}

// TestReplace_PermissionError verifies that attempting to replace a binary in
// a read-only directory returns an error with actionable guidance.
// R13: on permission error, print guidance message.
func TestReplace_PermissionError(t *testing.T) {
	t.Parallel()

	// Given: a target path inside a read-only directory
	readOnlyDir := t.TempDir()
	targetPath := filepath.Join(readOnlyDir, "autopus-adk")
	require.NoError(t, os.WriteFile(targetPath, []byte("original"), 0755))
	require.NoError(t, os.Chmod(readOnlyDir, 0555))
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0755) })

	newBinaryPath := filepath.Join(t.TempDir(), "autopus-adk-new")
	require.NoError(t, os.WriteFile(newBinaryPath, []byte("new content"), 0755))

	// When: Replace is called on a read-only directory
	r := NewReplacer()
	err := r.Replace(newBinaryPath, targetPath)

	// Then: error is returned containing guidance
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission")
}
