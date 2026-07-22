package content_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAntigravityStopHook_NextRoundInputUsesContinueJSON(t *testing.T) {
	run := runRoundStopHook(t, "gemini", "hook-gemini-stop.sh", "Continue in Antigravity.", false)
	var decision struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal([]byte(run.stdout), &decision))
	assert.Equal(t, "continue", decision.Decision)
	assert.Equal(t, "Continue in Antigravity.", decision.Reason)
	assertRoundArtifacts(t, run)
}

func TestAntigravityStopHook_NonContinuingPathsEmitStopJSON(t *testing.T) {
	t.Run("abort", func(t *testing.T) {
		run := runRoundStopHook(t, "gemini", "hook-gemini-stop.sh", "ignored", true)
		assertAntigravityStopDecision(t, run.stdout)
		assertRoundArtifacts(t, run)
	})
	t.Run("unscoped completion", func(t *testing.T) {
		run := runUnscopedStopHook(t, "gemini", "hook-gemini-stop.sh")
		assertAntigravityStopDecision(t, run.stdout)
	})
	t.Run("unmanaged session", func(t *testing.T) {
		cmd := exec.Command("sh", hookContractScript(t, "hook-gemini-stop.sh"))
		cmd.Stdin = strings.NewReader(`{"last_assistant_message":"ignored"}`)
		cmd.Env = append(os.Environ(), "AUTOPUS_SESSION_ID=", "AUTOPUS_SESSION_DIR=")
		stdout, err := cmd.Output()
		require.NoError(t, err)
		assertAntigravityStopDecision(t, string(stdout))
	})
}

func TestAntigravityStopHook_MalformedInputStillEmitsStopJSON(t *testing.T) {
	sessionDir := t.TempDir()
	cmd := exec.Command("sh", hookContractScript(t, "hook-gemini-stop.sh"))
	cmd.Stdin = strings.NewReader(`{`)
	cmd.Env = append(os.Environ(),
		"AUTOPUS_SESSION_ID=malformed",
		"AUTOPUS_SESSION_DIR="+sessionDir,
		"AUTOPUS_ROUND=",
	)
	stdout, err := cmd.Output()
	require.NoError(t, err)
	assertAntigravityStopDecision(t, string(stdout))
	assert.FileExists(t, filepath.Join(sessionDir, "gemini-result.json"))
	assert.FileExists(t, filepath.Join(sessionDir, "gemini-done"))
}

func TestAntigravityStopHook_CollectsActualCamelCaseTranscriptPayload(t *testing.T) {
	sessionDir := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	lines := strings.Join([]string{
		`{"source":"MODEL","type":"VIEW_FILE","content":"tool output"}`,
		`{"source":"MODEL","type":"PLANNER_RESPONSE","content":"final answer"}`,
	}, "\n") + "\n"
	require.NoError(t, os.WriteFile(transcript, []byte(lines), 0o600))
	payload, err := json.Marshal(map[string]any{
		"conversationId": "conversation",
		"transcriptPath": transcript,
		"fullyIdle":      true,
	})
	require.NoError(t, err)

	cmd := exec.Command("sh", hookContractScript(t, "hook-gemini-stop.sh"))
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Env = stopHookEnvironment(t, sessionDir, "")
	stdout, err := cmd.Output()
	require.NoError(t, err)
	assertAntigravityStopDecision(t, string(stdout))
	result := requireReadFile(t, filepath.Join(sessionDir, "gemini-result.json"))
	var completion struct {
		Output string `json:"output"`
	}
	require.NoError(t, json.Unmarshal(result, &completion))
	assert.Equal(t, "final answer", completion.Output)
}

func assertAntigravityStopDecision(t *testing.T, stdout string) {
	t.Helper()
	assert.Equal(t, `{"decision":"stop"}`, strings.TrimSpace(stdout))
}

func assertNoContinuation(t *testing.T, hook cursorHookCase, stdout string) {
	t.Helper()
	if hook.decision == "continue" {
		assertAntigravityStopDecision(t, stdout)
		return
	}
	assert.Empty(t, stdout)
}
