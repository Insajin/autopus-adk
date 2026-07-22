package content_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cursorHookCase struct {
	provider   string
	script     string
	runtime    string
	payloadKey string
	decision   string
}

type cursorHookSession struct {
	id  string
	dir string
}

type hookProcessResult struct {
	stdout string
	stderr string
}

func TestCompletionHooks_SuppressCmuxNoise(t *testing.T) {
	for _, hook := range cursorHookCases()[:3] {
		t.Run(hook.provider, func(t *testing.T) {
			session := newCursorHookSession(t, hook)
			writeRoundInput(t, session.dir, hook.provider, 2, "continue safely")
			result := runCursorHook(t, hook, session, "first output", true)
			assertContinuation(t, hook, result.stdout, "continue safely")
			assert.Empty(t, result.stderr, "cmux stderr must not escape the hook")
			assert.FileExists(t, filepath.Join(session.dir, "fake-cmux-called"))
		})
	}
}

func TestCompletionHooks_RoundCursorSurvivesFixedParentRound(t *testing.T) {
	for _, hook := range cursorHookCases() {
		t.Run(hook.provider, func(t *testing.T) {
			session := newCursorHookSession(t, hook)
			writeRoundInput(t, session.dir, hook.provider, 2, "round two prompt")

			first := runCursorHook(t, hook, session, "first output", false)
			assertContinuation(t, hook, first.stdout, "round two prompt")
			assert.Empty(t, first.stderr)
			assertHookResult(t, session.dir, hook.provider, 1, "first output")
			firstResult, err := os.ReadFile(roundResultPath(session.dir, hook.provider, 1))
			require.NoError(t, err)
			assert.FileExists(t, roundDonePath(session.dir, hook.provider, 1))

			cursorPath := filepath.Join(session.dir, hook.provider+"-round-cursor")
			assertCursor(t, cursorPath, 2)
			// Readers must accept the canonical ASCII decimal with a trailing newline.
			require.NoError(t, os.WriteFile(cursorPath, []byte("2\n"), 0o600))

			abortPath := filepath.Join(session.dir, hook.provider+"-round3-abort")
			require.NoError(t, os.WriteFile(abortPath, nil, 0o600))
			second := runCursorHook(t, hook, session, "second output", false)
			assertNoContinuation(t, hook, second.stdout)
			assert.Empty(t, second.stderr)
			assertHookResult(t, session.dir, hook.provider, 2, "second output")
			assert.FileExists(t, roundDonePath(session.dir, hook.provider, 2))
			assert.NoFileExists(t, abortPath)
			assert.NoFileExists(t, filepath.Join(session.dir, hook.provider+"-round3-ready"))
			assertCursor(t, cursorPath, 2)

			gotFirstResult, err := os.ReadFile(roundResultPath(session.dir, hook.provider, 1))
			require.NoError(t, err)
			assert.Equal(t, firstResult, gotFirstResult, "round1 result must not be overwritten")
		})
	}
}

func TestCompletionHooks_EmptyPromptDoesNotAdvanceCursor(t *testing.T) {
	for _, hook := range cursorHookCases() {
		t.Run(hook.provider, func(t *testing.T) {
			session := newCursorHookSession(t, hook)
			writeRoundInput(t, session.dir, hook.provider, 2, "")
			result := runCursorHook(t, hook, session, "first output", false)
			assertNoContinuation(t, hook, result.stdout)
			assert.Empty(t, result.stderr)
			assert.NoFileExists(t, filepath.Join(session.dir, hook.provider+"-round-cursor"))
		})
	}
}

func TestCompletionHooks_UseEnvironmentWhenItIsAheadOfCursor(t *testing.T) {
	for _, hook := range cursorHookCases() {
		t.Run(hook.provider, func(t *testing.T) {
			session := newCursorHookSession(t, hook)
			cursor := filepath.Join(session.dir, hook.provider+"-round-cursor")
			require.NoError(t, os.WriteFile(cursor, []byte("1\n"), 0o600))
			abort := filepath.Join(session.dir, hook.provider+"-round4-abort")
			require.NoError(t, os.WriteFile(abort, nil, 0o600))
			result := runCursorHookAtRound(t, hook, session, "third output", false, "3")
			assertNoContinuation(t, hook, result.stdout)
			assert.Empty(t, result.stderr)
			assertHookResult(t, session.dir, hook.provider, 3, "third output")
			assert.NoFileExists(t, abort)
			assertCursor(t, cursor, 1)
		})
	}
}

func cursorHookCases() []cursorHookCase {
	return []cursorHookCase{
		{provider: "claude", script: "hook-claude-stop.sh", runtime: "sh", payloadKey: "last_assistant_message", decision: "block"},
		{provider: "codex", script: "hook-codex-stop.sh", runtime: "sh", payloadKey: "last_assistant_message", decision: "block"},
		{provider: "gemini", script: "hook-gemini-afteragent.sh", runtime: "sh", payloadKey: "prompt_response", decision: "block"},
		{provider: "gemini", script: "hook-gemini-stop.sh", runtime: "sh", payloadKey: "last_assistant_message", decision: "continue"},
		{provider: "opencode", script: "hook-opencode-complete.ts", runtime: "bun", payloadKey: "text"},
	}
}

func newCursorHookSession(t *testing.T, hook cursorHookCase) cursorHookSession {
	t.Helper()
	return cursorHookSession{id: "cursor-" + hook.provider, dir: t.TempDir()}
}

func writeRoundInput(t *testing.T, dir, provider string, round int, prompt string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"provider": provider,
		"round":    round,
		"prompt":   prompt,
	})
	require.NoError(t, err)
	path := filepath.Join(dir, fmt.Sprintf("%s-round%d-input.json", provider, round))
	require.NoError(t, os.WriteFile(path, body, 0o600))
}

func runCursorHook(
	t *testing.T,
	hook cursorHookCase,
	session cursorHookSession,
	output string,
	noisyCmux bool,
) hookProcessResult {
	return runCursorHookAtRound(t, hook, session, output, noisyCmux, "1")
}

func runCursorHookAtRound(
	t *testing.T,
	hook cursorHookCase,
	session cursorHookSession,
	output string,
	noisyCmux bool,
	envRound string,
) hookProcessResult {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX hook contract")
	}
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	script := filepath.Clean(filepath.Join(
		filepath.Dir(thisFile), "..", "..", "content", "hooks", hook.script,
	))
	command := "sh"
	if hook.runtime == "bun" {
		var err error
		command, err = exec.LookPath("bun")
		if err != nil {
			t.Skip("bun is required for the OpenCode hook contract test")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, script)
	payload, err := json.Marshal(map[string]string{hook.payloadKey: output})
	require.NoError(t, err)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = cursorHookEnvironment(t, hook, session, noisyCmux, envRound)
	err = cmd.Run()
	if ctx.Err() != nil {
		require.FailNow(t, "hook timed out", "%s did not find the effective next-round signal", hook.provider)
	}
	require.NoError(t, err, "stderr: %s", stderr.String())
	return hookProcessResult{stdout: stdout.String(), stderr: stderr.String()}
}

func cursorHookEnvironment(
	t *testing.T,
	hook cursorHookCase,
	session cursorHookSession,
	noisyCmux bool,
	envRound string,
) []string {
	t.Helper()
	binDir := t.TempDir()
	for _, name := range []string{"python3", "chmod", "rm"} {
		path, err := exec.LookPath(name)
		require.NoError(t, err)
		require.NoError(t, os.Symlink(path, filepath.Join(binDir, name)))
	}
	if hook.runtime == "sh" {
		body := "#!/bin/sh\n: > \"$FAKE_CMUX_MARKER\"\n"
		if noisyCmux {
			body += "printf 'OK\\n'\nprintf 'ERR\\n' >&2\n"
		}
		cmux := filepath.Join(binDir, "cmux")
		require.NoError(t, os.WriteFile(cmux, []byte(body), 0o700))
	}
	env := make([]string, 0, len(os.Environ())+6)
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		switch key {
		case "PATH", "AUTOPUS_SESSION_ID", "AUTOPUS_SESSION_DIR", "AUTOPUS_ROUND", "FAKE_CMUX_MARKER":
			continue
		}
		env = append(env, entry)
	}
	return append(env,
		"PATH="+binDir,
		"AUTOPUS_SESSION_ID="+session.id,
		"AUTOPUS_SESSION_DIR="+session.dir,
		"AUTOPUS_ROUND="+envRound,
		"FAKE_CMUX_MARKER="+filepath.Join(session.dir, "fake-cmux-called"),
	)
}

func assertContinuation(t *testing.T, hook cursorHookCase, stdout, prompt string) {
	t.Helper()
	if hook.decision == "" {
		assert.Equal(t, prompt, stdout)
		return
	}
	var decision struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &decision), "stdout: %q", stdout)
	assert.Equal(t, hook.decision, decision.Decision)
	assert.Equal(t, prompt, decision.Reason)
}

func assertHookResult(t *testing.T, dir, provider string, round int, want string) {
	t.Helper()
	body, err := os.ReadFile(roundResultPath(dir, provider, round))
	require.NoError(t, err)
	var result struct {
		Output string `json:"output"`
	}
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, want, result.Output)
}

func assertCursor(t *testing.T, path string, want int) {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprint(want), strings.TrimSpace(string(body)))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func roundResultPath(dir, provider string, round int) string {
	return filepath.Join(dir, fmt.Sprintf("%s-round%d-result.json", provider, round))
}

func roundDonePath(dir, provider string, round int) string {
	return filepath.Join(dir, fmt.Sprintf("%s-round%d-done", provider, round))
}
