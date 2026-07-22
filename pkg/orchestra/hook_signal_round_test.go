package orchestra

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWaitForDoneRound_DetectsRoundFile verifies round-scoped done detection.
func TestWaitForDoneRound_DetectsRoundFile(t *testing.T) {
	t.Parallel()
	sess, err := NewHookSession("test-wait-done-round")
	require.NoError(t, err)
	defer sess.Cleanup()

	go func() {
		time.Sleep(100 * time.Millisecond)
		fname := RoundSignalName("claude", 2, "done")
		_ = os.WriteFile(filepath.Join(sess.Dir(), fname), []byte("1"), 0o644)
	}()

	err = sess.WaitForDoneRound(2*time.Second, "claude", 2)
	assert.NoError(t, err)
}

// TestWaitForDoneRound_Timeout verifies timeout on missing round done file.
func TestWaitForDoneRound_Timeout(t *testing.T) {
	t.Parallel()
	sess, err := NewHookSession("test-wait-done-round-timeout")
	require.NoError(t, err)
	defer sess.Cleanup()

	err = sess.WaitForDoneRound(300*time.Millisecond, "claude", 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

// TestWaitForDoneRound_ZeroRound_FallsBack verifies round=0 falls back to WaitForDone.
func TestWaitForDoneRound_ZeroRound_FallsBack(t *testing.T) {
	t.Parallel()
	sess, err := NewHookSession("test-wait-done-round-zero")
	require.NoError(t, err)
	defer sess.Cleanup()

	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(sess.Dir(), "claude-done"), []byte("1"), 0o644)
	}()

	err = sess.WaitForDoneRound(2*time.Second, "claude", 0)
	assert.NoError(t, err)
}

// TestReadResultRound_ValidJSON verifies round-scoped result reading.
func TestReadResultRound_ValidJSON(t *testing.T) {
	t.Parallel()
	sess, err := NewHookSession("test-read-result-round")
	require.NoError(t, err)
	defer sess.Cleanup()

	fname := RoundSignalName("gemini", 3, "result.json")
	data, _ := json.Marshal(HookResult{Output: "round 3 output", ExitCode: 0})
	require.NoError(t, os.WriteFile(filepath.Join(sess.Dir(), fname), data, 0o644))

	result, err := sess.ReadResultRound("gemini", 3)
	require.NoError(t, err)
	assert.Equal(t, "round 3 output", result.Output)
	assert.Equal(t, 0, result.ExitCode)
}

// TestReadResultRound_MissingFile verifies error on missing round result.
func TestReadResultRound_MissingFile(t *testing.T) {
	t.Parallel()
	sess, err := NewHookSession("test-read-result-round-missing")
	require.NoError(t, err)
	defer sess.Cleanup()

	_, err = sess.ReadResultRound("claude", 7)
	assert.Error(t, err)
}

// TestReadResultRound_ZeroRound_FallsBack verifies round=0 uses standard result file.
func TestReadResultRound_ZeroRound_FallsBack(t *testing.T) {
	t.Parallel()
	sess, err := NewHookSession("test-read-result-round-zero")
	require.NoError(t, err)
	defer sess.Cleanup()

	data, _ := json.Marshal(HookResult{Output: "standard result", ExitCode: 0})
	require.NoError(t, os.WriteFile(filepath.Join(sess.Dir(), "claude-result.json"), data, 0o644))

	result, err := sess.ReadResultRound("claude", 0)
	require.NoError(t, err)
	assert.Equal(t, "standard result", result.Output)
}

// TestReadResultRound_InvalidJSON verifies error on malformed JSON.
func TestReadResultRound_InvalidJSON(t *testing.T) {
	t.Parallel()
	sess, err := NewHookSession("test-read-result-round-badjson")
	require.NoError(t, err)
	defer sess.Cleanup()

	fname := RoundSignalName("claude", 1, "result.json")
	require.NoError(t, os.WriteFile(filepath.Join(sess.Dir(), fname), []byte("{bad}"), 0o644))

	_, err = sess.ReadResultRound("claude", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse result json")
}

// TestCollectRoundHookResults_Success verifies collecting results for a round.
func TestCollectRoundHookResults_Success(t *testing.T) {
	t.Parallel()
	sess, err := NewHookSession("test-collect-round-hook")
	require.NoError(t, err)
	defer sess.Cleanup()

	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{{Name: "claude"}, {Name: "gemini"}},
		TimeoutSeconds: 5,
	}

	// Write round 1 done + result files for both providers.
	for _, p := range []string{"claude", "gemini"} {
		doneName := RoundSignalName(p, 1, "done")
		resultName := RoundSignalName(p, 1, "result.json")
		_ = os.WriteFile(filepath.Join(sess.Dir(), doneName), []byte("1"), 0o644)
		data, _ := json.Marshal(HookResult{Output: p + " round 1 output", ExitCode: 0})
		_ = os.WriteFile(filepath.Join(sess.Dir(), resultName), data, 0o644)
	}

	responses := collectRoundHookResults(context.Background(), cfg, sess, 1)
	assert.Len(t, responses, 2)
	for _, r := range responses {
		assert.Contains(t, r.Output, "round 1 output")
		assert.False(t, r.TimedOut)
	}
}

// TestCollectRoundHookResults_Timeout verifies timeout for missing done files.
func TestCollectRoundHookResults_Timeout(t *testing.T) {
	t.Parallel()
	sess, err := NewHookSession("test-collect-round-timeout")
	require.NoError(t, err)
	defer sess.Cleanup()

	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{{Name: "claude"}},
		TimeoutSeconds: 1,
	}

	responses := collectRoundHookResults(context.Background(), cfg, sess, 99)
	require.Len(t, responses, 1)
	assert.True(t, responses[0].TimedOut)
}

func TestCollectRoundHookResults_UsesProviderExecutionTimeout(t *testing.T) {
	t.Parallel()
	sess, err := NewHookSession("test-collect-round-provider-timeout")
	require.NoError(t, err)
	defer sess.Cleanup()

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{{
			Name:             "claude",
			ExecutionTimeout: 200 * time.Millisecond,
		}},
		TimeoutSeconds: 5,
	}

	start := time.Now()
	responses := collectRoundHookResults(context.Background(), cfg, sess, 99)
	require.Len(t, responses, 1)
	assert.True(t, responses[0].TimedOut)
	assert.Less(t, time.Since(start), 1500*time.Millisecond, "provider timeout must isolate hook hangs from the global timeout")
}

// TestCollectRoundHookResults_MissingResult verifies graceful handling when
// done file exists but result file does not.
func TestCollectRoundHookResults_MissingResult(t *testing.T) {
	t.Parallel()
	sess, err := NewHookSession("test-collect-round-no-result")
	require.NoError(t, err)
	defer sess.Cleanup()

	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{{Name: "claude"}},
		TimeoutSeconds: 5,
	}

	doneName := RoundSignalName("claude", 1, "done")
	_ = os.WriteFile(filepath.Join(sess.Dir(), doneName), []byte("1"), 0o644)

	responses := collectRoundHookResults(context.Background(), cfg, sess, 1)
	require.Len(t, responses, 1)
	assert.Equal(t, "", responses[0].Output) // empty output on read failure
	assert.False(t, responses[0].TimedOut)
}

// TestSendRoundEnvToPane verifies that SendRoundEnvToPane sends the correct command.
func TestSendRoundEnvToPane(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	paneID, err := mock.SplitPane(context.Background(), 0) // 0 = Horizontal
	require.NoError(t, err)

	err = SendRoundEnvToPane(context.Background(), mock, paneID, 3)
	require.NoError(t, err)

	mock.mu.Lock()
	defer mock.mu.Unlock()
	require.Len(t, mock.sendCommandCalls, 1)
	assert.Equal(t, "export AUTOPUS_ROUND=3", mock.sendCommandCalls[0].Cmd)
}

// TestCollectRoundHookResults_ContextCancelled verifies that cancellation
// preserves one unavailable result per hook provider without waiting.
func TestCollectRoundHookResults_ContextCancelled(t *testing.T) {
	t.Parallel()
	sess, err := NewHookSession("test-collect-ctx-cancel")
	require.NoError(t, err)
	defer sess.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{{Name: "claude"}, {Name: "gemini"}},
		TimeoutSeconds: 5,
	}

	responses := collectRoundHookResults(ctx, cfg, sess, 1)
	require.Len(t, responses, 2)
	for _, response := range responses {
		assert.True(t, response.TimedOut)
		assert.True(t, response.EmptyOutput)
		assert.Contains(t, response.Error, context.Canceled.Error())
		assert.Equal(t, paneBackendName, response.ExecutedBackend)
	}
}

// TestCollectRoundHookResults_DefaultTimeout verifies the 60s default timeout
// is used when TimeoutSeconds is 0.
func TestCollectRoundHookResults_DefaultTimeout(t *testing.T) {
	t.Parallel()
	sess, err := NewHookSession("test-collect-default-timeout")
	require.NoError(t, err)
	defer sess.Cleanup()

	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{{Name: "claude"}},
		TimeoutSeconds: 0, // uses default 60s
	}

	// Write the done and result file immediately.
	doneName := RoundSignalName("claude", 1, "done")
	resultName := RoundSignalName("claude", 1, "result.json")
	data, _ := json.Marshal(HookResult{Output: "default timeout", ExitCode: 0})
	_ = os.WriteFile(filepath.Join(sess.Dir(), doneName), []byte("1"), 0o644)
	_ = os.WriteFile(filepath.Join(sess.Dir(), resultName), data, 0o644)

	responses := collectRoundHookResults(context.Background(), cfg, sess, 1)
	require.Len(t, responses, 1)
	assert.Equal(t, "default timeout", responses[0].Output)
}
