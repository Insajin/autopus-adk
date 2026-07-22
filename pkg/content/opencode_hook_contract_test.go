package content_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenCodeCompletionHook_RejectsUnsafeExportedSessionDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("filesystem hook contract")
	}
	realDir := t.TempDir()
	linkDir := filepath.Join(t.TempDir(), "session-link")
	require.NoError(t, os.Symlink(realDir, linkDir))
	notDir := filepath.Join(t.TempDir(), "session-file")
	require.NoError(t, os.WriteFile(notDir, []byte("not a directory"), 0o600))
	relativeRoot := t.TempDir()
	relativeDir := "relative-session"
	require.NoError(t, os.Mkdir(filepath.Join(relativeRoot, relativeDir), 0o700))

	for _, unsafeDir := range []string{linkDir, notDir} {
		runOpenCodeHook(t, "unsafe-opencode-session", unsafeDir, "")
	}
	runOpenCodeHook(t, "unsafe-opencode-session", relativeDir, relativeRoot)
	assert.NoFileExists(t, filepath.Join(realDir, "opencode-result.json"))
	assert.NoFileExists(t, filepath.Join(relativeRoot, relativeDir, "opencode-result.json"))
	assert.Equal(t, "not a directory", string(requireReadFile(t, notDir)))
}

func TestOpenCodeCompletionHook_UsesLegacySessionFallback(t *testing.T) {
	require.NoError(t, os.MkdirAll("/tmp/autopus", 0o700))
	dir, err := os.MkdirTemp("/tmp/autopus", "opencode-fallback-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	runOpenCodeHook(t, filepath.Base(dir), "", "")
	result := requireReadFile(t, filepath.Join(dir, "opencode-result.json"))
	assert.JSONEq(t, `{"output":"hook output","exit_code":0}`, string(result))
	assert.FileExists(t, filepath.Join(dir, "opencode-done"))
}

func runOpenCodeHook(t *testing.T, sessionID, sessionDir, workDir string) {
	t.Helper()
	bun, err := exec.LookPath("bun")
	if err != nil {
		t.Skip("bun is required for the OpenCode hook contract test")
	}
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	script := filepath.Clean(filepath.Join(
		filepath.Dir(thisFile), "..", "..", "content", "hooks", "hook-opencode-complete.ts",
	))
	cmd := exec.Command(bun, script)
	cmd.Dir = workDir
	cmd.Stdin = bytes.NewBufferString(`{"text":"hook output"}`)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	env := make([]string, 0, len(os.Environ())+3)
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		if key != "AUTOPUS_SESSION_ID" && key != "AUTOPUS_SESSION_DIR" && key != "AUTOPUS_ROUND" {
			env = append(env, entry)
		}
	}
	cmd.Env = append(env, "AUTOPUS_SESSION_ID="+sessionID, "AUTOPUS_ROUND=")
	if sessionDir != "" {
		cmd.Env = append(cmd.Env, "AUTOPUS_SESSION_DIR="+sessionDir)
	}
	require.NoError(t, cmd.Run(), "stderr: %s", stderr.String())
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

func requireReadFile(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	return body
}
