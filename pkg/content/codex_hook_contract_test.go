package content_test

import (
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

func TestCodexStopHook_AlwaysWritesResultBeforeDone(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	script := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "content", "hooks", "hook-codex-stop.sh"))

	for _, tt := range []struct {
		name    string
		payload string
	}{
		{name: "empty payload", payload: ""},
		{name: "invalid payload", payload: "not-json"},
		{name: "null payload", payload: "null"},
		{name: "array payload", payload: "[]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sessionDir := t.TempDir()
			cmd := exec.Command("sh", script)
			cmd.Stdin = strings.NewReader(tt.payload)
			cmd.Env = append(os.Environ(),
				"AUTOPUS_SESSION_ID=codex-hook-contract",
				"AUTOPUS_SESSION_DIR="+sessionDir,
				"AUTOPUS_ROUND=",
			)

			combined, err := cmd.CombinedOutput()
			require.NoError(t, err, "completion signaling is best-effort and must not fail: %s", combined)

			resultPath := filepath.Join(sessionDir, "codex-result.json")
			resultData, err := os.ReadFile(resultPath)
			require.NoError(t, err, "result file must be materialized even when the payload has no message")
			var result struct {
				Output   string `json:"output"`
				ExitCode int    `json:"exit_code"`
			}
			require.NoError(t, json.Unmarshal(resultData, &result))
			assert.Empty(t, result.Output)
			assert.Zero(t, result.ExitCode)

			_, err = os.Stat(filepath.Join(sessionDir, "codex-done"))
			assert.NoError(t, err, "done must always follow the best-effort result write")
		})
	}
}

func TestCodexStopHook_DoesNotFollowResultDoneOrReadySymlinks(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	script := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "content", "hooks", "hook-codex-stop.sh"))
	sessionDir := t.TempDir()
	victimDir := t.TempDir()

	links := map[string]string{
		"codex-round1-result.json": filepath.Join(victimDir, "result-victim"),
		"codex-round1-done":        filepath.Join(victimDir, "done-victim"),
		"codex-round2-ready":       filepath.Join(victimDir, "ready-victim"),
	}
	for linkName, victim := range links {
		require.NoError(t, os.WriteFile(victim, []byte("preserve-"+linkName), 0o600))
		if err := os.Symlink(victim, filepath.Join(sessionDir, linkName)); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
	}
	// Unblock the round-two input loop immediately after the ready signal.
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "codex-round2-abort"), nil, 0o600))

	cmd := exec.Command("sh", script)
	cmd.Stdin = strings.NewReader(`{"last_assistant_message":"safe output"}`)
	cmd.Env = append(os.Environ(),
		"AUTOPUS_SESSION_ID=codex-hook-symlink",
		"AUTOPUS_SESSION_DIR="+sessionDir,
		"AUTOPUS_ROUND=1",
	)
	combined, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", combined)

	for linkName, victim := range links {
		victimData, readErr := os.ReadFile(victim)
		require.NoError(t, readErr)
		assert.Equal(t, "preserve-"+linkName, string(victimData))
		if linkName == "codex-round2-ready" {
			_, err := os.Lstat(filepath.Join(sessionDir, linkName))
			assert.ErrorIs(t, err, os.ErrNotExist, "ready signal is removed after abort")
			continue
		}
		info, err := os.Lstat(filepath.Join(sessionDir, linkName))
		require.NoError(t, err)
		assert.Zero(t, info.Mode()&os.ModeSymlink)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestCodexSessionStartHook_DoesNotFollowReadySymlink(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	script := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "content", "hooks", "hook-codex-sessionstart.sh"))
	sessionDir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "ready-victim")
	require.NoError(t, os.WriteFile(victim, []byte("preserve-ready"), 0o600))
	readyPath := filepath.Join(sessionDir, "codex-round0-ready")
	if err := os.Symlink(victim, readyPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	cmd := exec.Command("sh", script)
	cmd.Env = append(os.Environ(),
		"AUTOPUS_SESSION_ID=codex-sessionstart-symlink",
		"AUTOPUS_SESSION_DIR="+sessionDir,
		"AUTOPUS_ROUND=0",
	)
	combined, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", combined)
	victimData, readErr := os.ReadFile(victim)
	require.NoError(t, readErr)
	assert.Equal(t, "preserve-ready", string(victimData))
	info, err := os.Lstat(readyPath)
	require.NoError(t, err)
	assert.Zero(t, info.Mode()&os.ModeSymlink)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestCodexHooks_RejectSymlinkSessionDirectory(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	hooksDir := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "content", "hooks"))
	realSessionDir := t.TempDir()
	linkRoot := t.TempDir()
	linkedSessionDir := filepath.Join(linkRoot, "session")
	if err := os.Symlink(realSessionDir, linkedSessionDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	for _, name := range []string{"hook-codex-stop.sh", "hook-codex-sessionstart.sh"} {
		cmd := exec.Command("sh", filepath.Join(hooksDir, name))
		cmd.Stdin = strings.NewReader(`{"last_assistant_message":"must not write"}`)
		cmd.Env = append(os.Environ(),
			"AUTOPUS_SESSION_ID=codex-linked-session",
			"AUTOPUS_SESSION_DIR="+linkedSessionDir,
			"AUTOPUS_ROUND=0",
		)
		combined, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s: %s", name, combined)
	}
	entries, err := os.ReadDir(realSessionDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestCodexStopHook_CollectsLatestAssistantMessageFromTranscript(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	script := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "content", "hooks", "hook-codex-stop.sh"))
	sessionDir := t.TempDir()
	transcriptPath := filepath.Join(t.TempDir(), "rollout.jsonl")
	transcript := strings.Join([]string{
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"older"}]}}`,
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"ignore"}]}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"{\"verdict\":\"PASS\"}"}]}}`,
	}, "\n") + "\n"
	require.NoError(t, os.WriteFile(transcriptPath, []byte(transcript), 0o600))
	payload, err := json.Marshal(map[string]any{"transcript_path": transcriptPath})
	require.NoError(t, err)

	cmd := exec.Command("sh", script)
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Env = append(os.Environ(),
		"AUTOPUS_SESSION_ID=codex-hook-transcript",
		"AUTOPUS_SESSION_DIR="+sessionDir,
		"AUTOPUS_ROUND=",
	)
	combined, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", combined)

	resultData, err := os.ReadFile(filepath.Join(sessionDir, "codex-result.json"))
	require.NoError(t, err)
	var result struct {
		Output string `json:"output"`
	}
	require.NoError(t, json.Unmarshal(resultData, &result))
	assert.JSONEq(t, `{"verdict":"PASS"}`, result.Output)
	assert.FileExists(t, filepath.Join(sessionDir, "codex-done"))
}

func TestCodexStopHook_DoesNotFollowTranscriptSymlink(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	script := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "content", "hooks", "hook-codex-stop.sh"))
	sessionDir := t.TempDir()
	transcriptTarget := filepath.Join(t.TempDir(), "rollout.jsonl")
	require.NoError(t, os.WriteFile(transcriptTarget, []byte(
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"secret"}]}}`+"\n",
	), 0o600))
	transcriptLink := filepath.Join(t.TempDir(), "linked-rollout.jsonl")
	if err := os.Symlink(transcriptTarget, transcriptLink); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	payload, err := json.Marshal(map[string]any{"transcript_path": transcriptLink})
	require.NoError(t, err)

	cmd := exec.Command("sh", script)
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Env = append(os.Environ(),
		"AUTOPUS_SESSION_ID=codex-transcript-symlink",
		"AUTOPUS_SESSION_DIR="+sessionDir,
		"AUTOPUS_ROUND=",
	)
	combined, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", combined)

	resultData, err := os.ReadFile(filepath.Join(sessionDir, "codex-result.json"))
	require.NoError(t, err)
	var result struct {
		Output string `json:"output"`
	}
	require.NoError(t, json.Unmarshal(resultData, &result))
	assert.Empty(t, result.Output)
	assert.FileExists(t, filepath.Join(sessionDir, "codex-done"))
}

func TestCodexStopHook_NextRoundInputUsesBlockJSON(t *testing.T) {
	assertStopHookNextRoundContract(t, "codex", "hook-codex-stop.sh")
}

func TestCodexStopHook_NonBlockingPathsEmitNoOutput(t *testing.T) {
	assertStopHookNonBlockingContract(t, "codex", "hook-codex-stop.sh")
}
