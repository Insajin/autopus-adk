package content_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stopHookRun struct {
	provider   string
	sessionDir string
	stdout     string
}

func TestClaudeStopHook_NextRoundInputUsesBlockJSON(t *testing.T) {
	assertStopHookNextRoundContract(t, "claude", "hook-claude-stop.sh")
}

func TestClaudeStopHook_NonBlockingPathsEmitNoOutput(t *testing.T) {
	assertStopHookNonBlockingContract(t, "claude", "hook-claude-stop.sh")
}

func assertStopHookNextRoundContract(t *testing.T, provider, scriptName string) {
	t.Helper()
	for _, tt := range []struct {
		name   string
		prompt string
	}{
		{name: "plain prompt", prompt: "Continue with round two."},
		{
			name:   "quotes newlines and control characters",
			prompt: "quote: \"double\" and slash: \\\nline two\tcarriage\rcontrol:\x01 nul:\x00\n끝 🐙\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			run := runRoundStopHook(t, provider, scriptName, tt.prompt, false)
			var decision struct {
				Decision string `json:"decision"`
				Reason   string `json:"reason"`
			}
			require.NoError(t, json.Unmarshal([]byte(run.stdout), &decision),
				"stdout must contain one valid Stop JSON object only: %q", run.stdout)
			assert.Equal(t, "block", decision.Decision)
			assert.Equal(t, tt.prompt, decision.Reason)
			assertRoundArtifacts(t, run)
		})
	}
}

func assertStopHookNonBlockingContract(t *testing.T, provider, scriptName string) {
	t.Helper()
	t.Run("abort", func(t *testing.T) {
		run := runRoundStopHook(t, provider, scriptName, "must not be emitted", true)
		assert.Empty(t, strings.TrimSpace(run.stdout), "abort must remain non-blocking")
		assertRoundArtifacts(t, run)
		assert.NoFileExists(t, filepath.Join(run.sessionDir, provider+"-round2-abort"))
	})
	t.Run("no next round", func(t *testing.T) {
		run := runUnscopedStopHook(t, provider, scriptName)
		assert.Empty(t, strings.TrimSpace(run.stdout), "an unscoped completion must not block Stop")
		assert.FileExists(t, filepath.Join(run.sessionDir, provider+"-result.json"))
		assert.FileExists(t, filepath.Join(run.sessionDir, provider+"-done"))
	})
}

func runRoundStopHook(t *testing.T, provider, scriptName, prompt string, abort bool) stopHookRun {
	t.Helper()
	sessionDir := t.TempDir()
	nextName := provider + "-round2-input.json"
	if abort {
		require.NoError(t, os.WriteFile(
			filepath.Join(sessionDir, provider+"-round2-abort"), nil, 0o600,
		))
	} else {
		input, err := json.Marshal(map[string]any{
			"provider": provider,
			"round":    2,
			"prompt":   prompt,
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(sessionDir, nextName), input, 0o600))
	}

	run := executeStopHook(t, provider, scriptName, sessionDir, "1")
	assert.NoFileExists(t, filepath.Join(sessionDir, nextName))
	assert.NoFileExists(t, filepath.Join(sessionDir, provider+"-round2-ready"))
	return run
}

func runUnscopedStopHook(t *testing.T, provider, scriptName string) stopHookRun {
	t.Helper()
	return executeStopHook(t, provider, scriptName, t.TempDir(), "")
}

func executeStopHook(t *testing.T, provider, scriptName, sessionDir, round string) stopHookRun {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX hook contract")
	}
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	script := filepath.Clean(filepath.Join(
		filepath.Dir(thisFile), "..", "..", "content", "hooks", scriptName,
	))
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("sh", script)
	cmd.Stdin = strings.NewReader(`{"last_assistant_message":"round one output"}`)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = stopHookEnvironment(t, sessionDir, round)
	require.NoError(t, cmd.Run(), "stderr: %s", stderr.String())
	return stopHookRun{provider: provider, sessionDir: sessionDir, stdout: stdout.String()}
}

func stopHookEnvironment(t *testing.T, sessionDir, round string) []string {
	t.Helper()
	binDir := t.TempDir()
	for _, name := range []string{"python3", "chmod", "rm"} {
		path, err := exec.LookPath(name)
		require.NoError(t, err)
		require.NoError(t, os.Symlink(path, filepath.Join(binDir, name)))
	}
	env := make([]string, 0, len(os.Environ())+4)
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		switch key {
		case "PATH", "AUTOPUS_SESSION_ID", "AUTOPUS_SESSION_DIR", "AUTOPUS_ROUND":
			continue
		}
		env = append(env, entry)
	}
	return append(env,
		"PATH="+binDir,
		"AUTOPUS_SESSION_ID=stop-hook-contract",
		"AUTOPUS_SESSION_DIR="+sessionDir,
		"AUTOPUS_ROUND="+round,
	)
}

func assertRoundArtifacts(t *testing.T, run stopHookRun) {
	t.Helper()
	resultData, err := os.ReadFile(filepath.Join(
		run.sessionDir, run.provider+"-round1-result.json",
	))
	require.NoError(t, err)
	var result struct {
		Output   string `json:"output"`
		ExitCode int    `json:"exit_code"`
	}
	require.NoError(t, json.Unmarshal(resultData, &result))
	assert.Equal(t, "round one output", result.Output)
	assert.Zero(t, result.ExitCode)
	assert.FileExists(t, filepath.Join(run.sessionDir, run.provider+"-round1-done"))
}
