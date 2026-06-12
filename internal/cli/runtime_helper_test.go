package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// TestAppendDurationFlag_IncludeAndExclude verifies conditional duration flag append.
func TestAppendDurationFlag_IncludeAndExclude(t *testing.T) {
	t.Parallel()

	base := []string{"--foo", "bar"}
	got := appendDurationFlag(base, "timeout", 30*time.Second, true)
	assert.Equal(t, []string{"--foo", "bar", "--timeout", "30s"}, got)

	got = appendDurationFlag(base, "timeout", 30*time.Second, false)
	assert.Equal(t, []string{"--foo", "bar"}, got)
}

// TestAppendStringFlag_IncludeAndExclude verifies conditional string flag append.
func TestAppendStringFlag_IncludeAndExclude(t *testing.T) {
	t.Parallel()

	got := appendStringFlag(nil, "model", "claude", true)
	assert.Equal(t, []string{"--model", "claude"}, got)

	got = appendStringFlag(nil, "model", "claude", false)
	assert.Empty(t, got)
}

// TestAppendBoolFlag_IncludeAndExclude verifies conditional bool flag append.
func TestAppendBoolFlag_IncludeAndExclude(t *testing.T) {
	t.Parallel()

	got := appendBoolFlag(nil, "json", true)
	assert.Equal(t, []string{"--json"}, got)

	got = appendBoolFlag(nil, "json", false)
	assert.Empty(t, got)
}

// TestRewriteRuntimeHelperEnvelope_InjectsCommandPath overwrites the command field.
func TestRewriteRuntimeHelperEnvelope_InjectsCommandPath(t *testing.T) {
	t.Parallel()

	payload := `{"command":"old","status":"ok","data":{"key":"value"}}`
	got, err := rewriteRuntimeHelperEnvelope([]byte(payload), "auto desktop auth import")
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(got, &out))
	assert.Equal(t, "auto desktop auth import", out["command"])
	assert.Equal(t, "ok", out["status"])
}

// TestRewriteRuntimeHelperEnvelope_InvalidJSON returns error on bad input.
func TestRewriteRuntimeHelperEnvelope_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := rewriteRuntimeHelperEnvelope([]byte(`{bad json`), "cmd")
	require.Error(t, err)
}

// TestIsExecutableFile_VariousCases checks executable bit, dirs, and missing files.
func TestIsExecutableFile_VariousCases(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	execPath := filepath.Join(dir, "bin")
	require.NoError(t, os.WriteFile(execPath, []byte("#!/bin/sh"), 0o755))
	assert.True(t, isExecutableFile(execPath))

	readOnlyPath := filepath.Join(dir, "ro")
	require.NoError(t, os.WriteFile(readOnlyPath, []byte("data"), 0o644))
	assert.False(t, isExecutableFile(readOnlyPath))

	assert.False(t, isExecutableFile(filepath.Join(dir, "nonexistent")))
	assert.False(t, isExecutableFile(dir))
}

// TestPackagedRuntimeHelperPatternsFrom_ContainsBinaryName verifies pattern names.
func TestPackagedRuntimeHelperPatternsFrom_ContainsBinaryName(t *testing.T) {
	t.Parallel()

	patterns := packagedRuntimeHelperPatternsFrom("/usr/local/bin")
	assert.NotEmpty(t, patterns)
	for _, p := range patterns {
		assert.Contains(t, p, runtimeHelperBinaryName)
	}
}

// TestMacOSRuntimeHelperPatterns_AbsolutePaths ensures all patterns are absolute.
func TestMacOSRuntimeHelperPatterns_AbsolutePaths(t *testing.T) {
	t.Parallel()

	patterns := macOSRuntimeHelperPatterns()
	assert.NotEmpty(t, patterns)
	for _, p := range patterns {
		assert.True(t, filepath.IsAbs(p), "pattern must be absolute: %s", p)
	}
}

// TestResolveRuntimeHelperOverride_RelativePathError rejects relative paths.
func TestResolveRuntimeHelperOverride_RelativePathError(t *testing.T) {
	t.Parallel()

	_, err := resolveRuntimeHelperOverride("relative/path/helper")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute")
}

// TestResolveRuntimeHelperOverride_MissingAbsPathError returns error for absent path.
func TestResolveRuntimeHelperOverride_MissingAbsPathError(t *testing.T) {
	t.Parallel()

	_, err := resolveRuntimeHelperOverride("/nonexistent/autopus-desktop-runtime")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing or non-executable")
}

// TestResolveRuntimeHelperOverride_ValidExecutable returns the cleaned path.
func TestResolveRuntimeHelperOverride_ValidExecutable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	bin := filepath.Join(dir, "myhelper")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh"), 0o755))

	got, err := resolveRuntimeHelperOverride(bin)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(bin), got)
}

// TestDefaultRuntimeHelperPackagedPatterns_ReturnsNonEmpty smoke-tests the generator.
func TestDefaultRuntimeHelperPackagedPatterns_ReturnsNonEmpty(t *testing.T) {
	t.Parallel()

	patterns := defaultRuntimeHelperPackagedPatterns()
	assert.NotEmpty(t, patterns)
}
