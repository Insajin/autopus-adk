package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeRuntimeHelperScript(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, runtimeHelperBinaryName)
	script := "#!/bin/sh\nset -eu\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

func stubRuntimeHelperPackagedPatterns(t *testing.T, patterns []string) {
	t.Helper()

	original := runtimeHelperPackagedPatterns
	runtimeHelperPackagedPatterns = func() []string {
		return patterns
	}
	t.Cleanup(func() {
		runtimeHelperPackagedPatterns = original
	})
}

func TestResolveRuntimeHelperBinary_UsesOverrideEnv(t *testing.T) {
	helperPath := writeRuntimeHelperScript(t, "printf 'helper'\n")
	t.Setenv(runtimeHelperOverrideEnv, helperPath)

	resolved, err := resolveRuntimeHelperBinary()
	require.NoError(t, err)
	assert.Equal(t, helperPath, resolved)
}

func TestResolveRuntimeHelperBinary_RejectsRelativeOverrideEnv(t *testing.T) {
	t.Setenv(runtimeHelperOverrideEnv, runtimeHelperBinaryName)

	_, err := resolveRuntimeHelperBinary()
	require.Error(t, err)
	assert.ErrorContains(t, err, "must be an absolute helper path")
}

func TestResolveRuntimeHelperBinary_IgnoresPathSearch(t *testing.T) {
	stubRuntimeHelperPackagedPatterns(t, nil)

	helperPath := writeRuntimeHelperScript(t, "printf 'helper'\n")
	t.Setenv(runtimeHelperOverrideEnv, "")
	t.Setenv("PATH", filepath.Dir(helperPath))

	_, err := resolveRuntimeHelperBinary()
	require.Error(t, err)
	assert.ErrorIs(t, err, errRuntimeHelperNotFound)
}

func TestResolveRuntimeHelperBinary_IgnoresCwdSiblingHelper(t *testing.T) {
	stubRuntimeHelperPackagedPatterns(t, nil)

	tempDir := t.TempDir()
	helperDir := filepath.Join(tempDir, "autopus-desktop", "src-tauri", "binaries")
	require.NoError(t, os.MkdirAll(helperDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(helperDir, runtimeHelperBinaryName), []byte("#!/bin/sh\n"), 0o755))

	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(wd))
	})

	t.Setenv(runtimeHelperOverrideEnv, "")
	t.Setenv("PATH", "")

	_, err = resolveRuntimeHelperBinary()
	require.Error(t, err)
	assert.ErrorIs(t, err, errRuntimeHelperNotFound)
}

func TestResolveRuntimeHelperBinary_ReportsMissingHelper(t *testing.T) {
	stubRuntimeHelperPackagedPatterns(t, nil)

	t.Setenv(runtimeHelperOverrideEnv, "")
	t.Setenv("PATH", "")

	wd, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(wd))
	})

	_, err = resolveRuntimeHelperBinary()
	require.Error(t, err)
	assert.ErrorIs(t, err, errRuntimeHelperNotFound)
	assert.ErrorContains(t, err, "install/update the desktop app")
	assert.ErrorContains(t, err, runtimeHelperOverrideEnv)
}
