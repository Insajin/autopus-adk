package content_test

import (
	"bytes"
	"context"
	"encoding/json"
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

const hookInputLimit = 1024 * 1024

func TestCompletionHooks_ArtifactSymlinksDoNotEscapeSession(t *testing.T) {
	for _, hook := range confinementHookCases() {
		t.Run(hook.provider, func(t *testing.T) {
			session := newCursorHookSession(t, hook)
			writeRoundInput(t, session.dir, hook.provider, 2, "confined continuation")
			victimDir := t.TempDir()
			names := []string{
				hook.provider + "-round1-result.json",
				hook.provider + "-round1-done",
				hook.provider + "-round2-ready",
				hook.provider + "-round-cursor",
			}
			victims := make(map[string]string, len(names))
			for _, name := range names {
				victim := filepath.Join(victimDir, name)
				require.NoError(t, os.WriteFile(victim, []byte("preserve-"+name), 0o600))
				require.NoError(t, os.Symlink(victim, filepath.Join(session.dir, name)))
				victims[name] = victim
			}

			result := runConfinementHook(t, hook, session.id, session.dir, "1", "round one", "")

			assertContinuation(t, hook, result.stdout, "confined continuation")
			assert.Empty(t, result.stderr)
			for name, victim := range victims {
				assert.Equal(t, "preserve-"+name, string(requireReadFile(t, victim)))
			}
			assertRegularMode(t, filepath.Join(session.dir, names[0]), 0o600)
			assertRegularMode(t, filepath.Join(session.dir, names[1]), 0o600)
			assertRegularMode(t, filepath.Join(session.dir, names[3]), 0o600)
			assert.NoFileExists(t, filepath.Join(session.dir, names[2]))
		})
	}
}

func TestCompletionHooks_InputSymlinkFailsClosed(t *testing.T) {
	for _, hook := range confinementHookCases() {
		t.Run(hook.provider, func(t *testing.T) {
			session := newCursorHookSession(t, hook)
			target := filepath.Join(t.TempDir(), "outside-input.json")
			body := hookInputJSON(t, hook.provider, 2, "must not continue")
			require.NoError(t, os.WriteFile(target, body, 0o600))
			input := filepath.Join(session.dir, hook.provider+"-round2-input.json")
			require.NoError(t, os.Symlink(target, input))

			result := runConfinementHook(t, hook, session.id, session.dir, "1", "round one", "")

			assertNoContinuation(t, hook, result.stdout)
			assert.Empty(t, result.stderr)
			assert.Equal(t, body, requireReadFile(t, target))
			assert.NoFileExists(t, filepath.Join(session.dir, hook.provider+"-round-cursor"))
		})
	}
}

func TestCompletionHooks_OversizedInputFailsClosed(t *testing.T) {
	for _, hook := range confinementHookCases() {
		t.Run(hook.provider, func(t *testing.T) {
			session := newCursorHookSession(t, hook)
			input := filepath.Join(session.dir, hook.provider+"-round2-input.json")
			body := hookInputJSON(t, hook.provider, 2, strings.Repeat("x", hookInputLimit+1))
			require.NoError(t, os.WriteFile(input, body, 0o600))

			result := runConfinementHook(t, hook, session.id, session.dir, "1", "round one", "")

			assertNoContinuation(t, hook, result.stdout)
			assert.Empty(t, result.stderr)
			assert.NoFileExists(t, filepath.Join(session.dir, hook.provider+"-round-cursor"))
		})
	}
}

func TestCompletionHooks_ArtifactWriteFailureFailsClosed(t *testing.T) {
	for _, hook := range confinementHookCases() {
		t.Run(hook.provider, func(t *testing.T) {
			for _, artifact := range []string{"result", "cursor"} {
				t.Run(artifact, func(t *testing.T) {
					session := newCursorHookSession(t, hook)
					writeRoundInput(t, session.dir, hook.provider, 2, "must not continue")
					blockedPath := roundResultPath(session.dir, hook.provider, 1)
					if artifact == "cursor" {
						blockedPath = filepath.Join(session.dir, hook.provider+"-round-cursor")
					}
					require.NoError(t, os.Mkdir(blockedPath, 0o700))

					result := runConfinementHook(t, hook, session.id, session.dir, "1", "round one", "")

					assertNoContinuation(t, hook, result.stdout)
					assert.Empty(t, result.stderr)
					assert.DirExists(t, blockedPath)
				})
			}
		})
	}
}

func confinementHookCases() []cursorHookCase {
	all := cursorHookCases()
	return []cursorHookCase{all[0], all[2], all[3], all[4]}
}

func hookInputJSON(t *testing.T, provider string, round int, prompt string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{"provider": provider, "round": round, "prompt": prompt})
	require.NoError(t, err)
	return body
}

func runConfinementHook(
	t *testing.T,
	hook cursorHookCase,
	sessionID, sessionDir, round, output, workDir string,
) hookProcessResult {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("filesystem hook contract")
	}
	command := "sh"
	if hook.runtime == "bun" {
		var err error
		command, err = exec.LookPath("bun")
		if err != nil {
			t.Skip("bun is required for the OpenCode hook contract test")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, hookContractScript(t, hook.script))
	cmd.Dir = workDir
	payload, err := json.Marshal(map[string]string{hook.payloadKey: output})
	require.NoError(t, err)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.Env = confinementHookEnvironment(sessionID, sessionDir, round)
	err = cmd.Run()
	require.NoError(t, ctx.Err(), "hook exceeded confinement test deadline")
	require.NoError(t, err, "stderr: %s", stderr.String())
	return hookProcessResult{stdout: stdout.String(), stderr: stderr.String()}
}

func confinementHookEnvironment(sessionID, sessionDir, round string) []string {
	env := make([]string, 0, len(os.Environ())+4)
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		switch key {
		case "PATH", "AUTOPUS_SESSION_ID", "AUTOPUS_SESSION_DIR", "AUTOPUS_ROUND":
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "PATH=/opt/homebrew/bin:/usr/bin:/bin", "AUTOPUS_SESSION_ID="+sessionID, "AUTOPUS_ROUND="+round)
	if sessionDir != "" {
		env = append(env, "AUTOPUS_SESSION_DIR="+sessionDir)
	}
	return env
}

func hookContractScript(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "content", "hooks", name))
}

func assertRegularMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular(), "%s must be a regular file", path)
	assert.Zero(t, info.Mode()&os.ModeSymlink)
	assert.Equal(t, want, info.Mode().Perm())
}
