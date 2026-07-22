package content_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionHooks_RejectSymlinkAndRelativeSessionDirectories(t *testing.T) {
	for _, hook := range confinementHookCases() {
		t.Run(hook.provider, func(t *testing.T) {
			t.Run("symlink", func(t *testing.T) {
				realDir := t.TempDir()
				linkedDir := filepath.Join(t.TempDir(), "session-link")
				require.NoError(t, os.Symlink(realDir, linkedDir))

				result := runConfinementHook(t, hook, "linked-session", linkedDir, "", "must not write", "")

				assertNoContinuation(t, hook, result.stdout)
				assert.Empty(t, result.stderr)
				assert.Empty(t, requireReadDir(t, realDir))
			})

			t.Run("relative", func(t *testing.T) {
				root := t.TempDir()
				require.NoError(t, os.Mkdir(filepath.Join(root, "relative-session"), 0o700))

				result := runConfinementHook(t, hook, "relative-session", "relative-session", "", "must not write", root)

				assertNoContinuation(t, hook, result.stdout)
				assert.Empty(t, result.stderr)
				assert.Empty(t, requireReadDir(t, filepath.Join(root, "relative-session")))
			})
		})
	}
}

func TestCompletionHooks_LegacySessionFallbackRemainsSupported(t *testing.T) {
	require.NoError(t, os.MkdirAll("/tmp/autopus", 0o700))
	for _, hook := range confinementHookCases() {
		t.Run(hook.provider, func(t *testing.T) {
			dir, err := os.MkdirTemp("/tmp/autopus", "confinement-"+hook.provider+"-")
			require.NoError(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(dir) })

			result := runConfinementHook(t, hook, filepath.Base(dir), "", "", "legacy output", "")

			assertNoContinuation(t, hook, result.stdout)
			assert.Empty(t, result.stderr)
			assertRegularMode(t, filepath.Join(dir, hook.provider+"-result.json"), 0o600)
			assertRegularMode(t, filepath.Join(dir, hook.provider+"-done"), 0o600)
		})
	}
}

func TestSessionStartHooks_ConfineReadyArtifact(t *testing.T) {
	for _, tc := range []struct {
		provider string
		script   string
	}{
		{provider: "claude", script: "hook-claude-sessionstart.sh"},
		{provider: "gemini", script: "hook-gemini-sessionstart.sh"},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			sessionDir := t.TempDir()
			victim := filepath.Join(t.TempDir(), "ready-victim")
			require.NoError(t, os.WriteFile(victim, []byte("preserve-ready"), 0o600))
			ready := filepath.Join(sessionDir, tc.provider+"-round1-ready")
			require.NoError(t, os.Symlink(victim, ready))

			result := runSessionStartHook(t, tc.script, sessionDir, "1")

			assert.Empty(t, result.stdout)
			assert.Empty(t, result.stderr)
			assert.Equal(t, "preserve-ready", string(requireReadFile(t, victim)))
			assertRegularMode(t, ready, 0o600)
		})
	}
}

func TestSessionStartHooks_RejectSymlinkSessionDirectory(t *testing.T) {
	for _, tc := range []struct {
		provider string
		script   string
	}{
		{provider: "claude", script: "hook-claude-sessionstart.sh"},
		{provider: "gemini", script: "hook-gemini-sessionstart.sh"},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			realDir := t.TempDir()
			linkedDir := filepath.Join(t.TempDir(), "session-link")
			require.NoError(t, os.Symlink(realDir, linkedDir))

			result := runSessionStartHook(t, tc.script, linkedDir, "1")

			assert.Empty(t, result.stdout)
			assert.Empty(t, result.stderr)
			assert.Empty(t, requireReadDir(t, realDir))
		})
	}
}

func runSessionStartHook(t *testing.T, script, sessionDir, round string) hookProcessResult {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("filesystem hook contract")
	}
	cmd := exec.Command("sh", hookContractScript(t, script))
	cmd.Env = confinementHookEnvironment("session-start", sessionDir, round)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	require.NoError(t, err, "stderr: %s", stderr.String())
	return hookProcessResult{stdout: stdout.String(), stderr: stderr.String()}
}

func requireReadDir(t *testing.T, path string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	return entries
}
