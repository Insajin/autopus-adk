package content_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexHooks_RejectRelativeSessionDirectory(t *testing.T) {
	for _, tc := range codexSessionDirectoryCases() {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			relativeDir := "relative-session"
			require.NoError(t, os.Mkdir(filepath.Join(root, relativeDir), 0o700))

			result := runCodexSessionDirectoryHook(t, tc, root, "codex-relative", relativeDir)

			assert.Empty(t, result.stdout)
			assert.Empty(t, result.stderr)
			assert.Empty(t, requireReadDir(t, filepath.Join(root, relativeDir)))
		})
	}
}

func TestCodexHooks_LegacyAbsoluteSessionFallback(t *testing.T) {
	require.NoError(t, os.MkdirAll("/tmp/autopus", 0o700))
	for _, tc := range codexSessionDirectoryCases() {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("/tmp/autopus", "codex-legacy-")
			require.NoError(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(dir) })

			result := runCodexSessionDirectoryHook(t, tc, "", filepath.Base(dir), "")

			assert.Empty(t, result.stdout)
			assert.Empty(t, result.stderr)
			for _, artifact := range tc.artifacts {
				assertRegularMode(t, filepath.Join(dir, artifact), 0o600)
			}
		})
	}
}

type codexSessionDirectoryCase struct {
	name      string
	script    string
	round     string
	payload   string
	artifacts []string
}

func codexSessionDirectoryCases() []codexSessionDirectoryCase {
	return []codexSessionDirectoryCase{
		{
			name: "stop", script: "hook-codex-stop.sh", payload: `{"last_assistant_message":"legacy"}`,
			artifacts: []string{"codex-result.json", "codex-done"},
		},
		{
			name: "sessionstart", script: "hook-codex-sessionstart.sh", round: "0",
			artifacts: []string{"codex-round0-ready"},
		},
	}
}

func runCodexSessionDirectoryHook(
	t *testing.T,
	tc codexSessionDirectoryCase,
	workDir, sessionID, sessionDir string,
) hookProcessResult {
	t.Helper()
	cmd := exec.Command("sh", hookContractScript(t, tc.script))
	cmd.Dir = workDir
	cmd.Stdin = bytes.NewBufferString(tc.payload)
	cmd.Env = confinementHookEnvironment(sessionID, sessionDir, tc.round)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	require.NoError(t, cmd.Run(), "stderr: %s", stderr.String())
	return hookProcessResult{stdout: stdout.String(), stderr: stderr.String()}
}
