package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveCurrentBinaryPath_ResolvesSymlinkTarget(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "auto-target")
	symlinkPath := filepath.Join(tmpDir, "auto-link")
	require.NoError(t, os.WriteFile(targetPath, []byte("bin"), 0o755))
	require.NoError(t, os.Symlink(targetPath, symlinkPath))

	originalExecutable := currentExecutablePath
	originalEval := evalBinarySymlinks
	t.Cleanup(func() {
		currentExecutablePath = originalExecutable
		evalBinarySymlinks = originalEval
	})

	currentExecutablePath = func() (string, error) { return symlinkPath, nil }
	evalBinarySymlinks = filepath.EvalSymlinks

	info, err := resolveCurrentBinaryPath()
	require.NoError(t, err)
	expectedPath, err := filepath.EvalSymlinks(targetPath)
	require.NoError(t, err)
	assert.Equal(t, symlinkPath, info.ExecutablePath)
	assert.Equal(t, expectedPath, info.ManagedPath())
	assert.True(t, info.IsSymlinked())
}

func TestResolveCurrentBinaryPath_FallsBackWhenSymlinkEvalFails(t *testing.T) {
	originalExecutable := currentExecutablePath
	originalEval := evalBinarySymlinks
	t.Cleanup(func() {
		currentExecutablePath = originalExecutable
		evalBinarySymlinks = originalEval
	})

	currentExecutablePath = func() (string, error) { return "/tmp/auto", nil }
	evalBinarySymlinks = func(string) (string, error) { return "", assert.AnError }

	info, err := resolveCurrentBinaryPath()
	require.NoError(t, err)
	assert.Equal(t, "/tmp/auto", info.ManagedPath())
	assert.False(t, info.IsSymlinked())
}
