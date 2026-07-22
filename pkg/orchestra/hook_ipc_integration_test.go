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

// TestTryFileIPC_Success verifies the delivered outcome when ready and input writes succeed.
func TestTryFileIPC_Success(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-try-file-ipc-ok")
	require.NoError(t, err)
	defer sess.Cleanup()

	// Given: ready file appears before timeout
	readyName := RoundSignalName("claude", 2, "ready")
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(sess.Dir(), readyName), []byte("1"), 0o644)
	}()

	// When: tryFileIPC is called
	ctx := context.Background()
	outcome, ipcErr := tryFileIPC(ctx, sess, "claude", 2, "debate prompt")

	// Then: file IPC was delivered without falling back.
	require.NoError(t, ipcErr)
	assert.Equal(t, fileIPCDelivered, outcome)

	// Then: input file was created
	inputName := RoundSignalName("claude", 2, "input.json")
	_, err = os.Stat(filepath.Join(sess.Dir(), inputName))
	assert.NoError(t, err, "input file must exist after successful IPC")
}

// TestTryFileIPC_ContextDeadlineReleaseFailure verifies that a cancelled caller
// cannot authorize direct fallback without a release acknowledgement.
func TestTryFileIPC_ContextDeadlineReleaseFailure(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-try-file-ipc-timeout")
	require.NoError(t, err)
	defer sess.Cleanup()

	// When: tryFileIPC is called but no ready file appears
	// Use a cancelled context to speed up the test
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	outcome, ipcErr := tryFileIPC(ctx, sess, "claude", 1, "prompt")

	// Then: cancellation prevents release acknowledgement and direct fallback.
	assert.Equal(t, fileIPCReleaseFailure, outcome)
	require.Error(t, ipcErr)
	abortPath := filepath.Join(sess.Dir(), RoundSignalName("claude", 1, "abort"))
	assert.FileExists(t, abortPath, "timeout fallback must release a late hook waiter")
}

// TestTryFileIPC_ContextCancelledReleaseFailure covers cancellation before ready wait.
func TestTryFileIPC_ContextCancelledReleaseFailure(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-try-file-ipc-ctx-cancel")
	require.NoError(t, err)
	defer sess.Cleanup()

	// Given: context is already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When: tryFileIPC is called
	outcome, ipcErr := tryFileIPC(ctx, sess, "claude", 1, "prompt")

	// Then: direct fallback is not authorized on a cancelled context.
	assert.Equal(t, fileIPCReleaseFailure, outcome)
	assert.ErrorIs(t, ipcErr, context.Canceled)
	assert.FileExists(t, filepath.Join(sess.Dir(), RoundSignalName("claude", 1, "abort")),
		"cancelled fallback must release the hook waiter")
}

// TestTryFileIPC_WriteFailure_SendsAbort verifies R5-SAFETY: when WriteInputRound
// fails after WaitForReady succeeds, an abort signal is written.
func TestTryFileIPC_WriteFailure_SendsAbort(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-try-file-ipc-write-fail")
	require.NoError(t, err)
	defer sess.Cleanup()

	// Given: ready file exists immediately
	readyName := RoundSignalName("claude", 1, "ready")
	require.NoError(t, os.WriteFile(filepath.Join(sess.Dir(), readyName), []byte("1"), 0o644))

	// Given: collide with the atomic input temp file while leaving abort writes available.
	inputName := RoundSignalName("claude", 1, "input.json")
	require.NoError(t, os.Mkdir(filepath.Join(sess.Dir(), inputName+".tmp"), 0o700))

	consumed := consumeFileIPCAbort(sess, "claude", 1, nil, nil)
	// When: tryFileIPC is called
	outcome, ipcErr := tryFileIPC(context.Background(), sess, "claude", 1, "prompt")

	// Then: fallback is safe only after the waiter consumes abort and ready.
	assert.Equal(t, fileIPCSafeFallback, outcome)
	assert.ErrorContains(t, ipcErr, "write input")
	require.NoError(t, <-consumed)
}

// TestExecuteRound_FileIPC_Path verifies that executeRound uses file IPC
// for hook providers in round 2+ when hookSession is active (SPEC-ORCH-017 R4).
func TestExecuteRound_FileIPC_Path(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.readScreenOutput = "hook is waiting; no prompt is visible\n"

	sess, err := NewHookSession("test-exec-round-fileipc-" + NewSessionID())
	require.NoError(t, err)
	defer sess.Cleanup()

	// Only claude has hook; gemini does not
	sess.SetHookProviders(map[string]bool{"claude": true})

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "echo", InteractiveInput: "stdin"},
		},
		Strategy:       StrategyDebate,
		Prompt:         "round 2 prompt",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		HookMode:       true,
		InitialDelay:   time.Millisecond,
	}

	panes := []paneInfo{
		{provider: cfg.Providers[0], paneID: "pane-1"},
	}

	// Simulate round 1 responses (needed for round 2 rebuttal)
	prevResponses := []ProviderResponse{
		{Provider: "claude", Output: "round 1 response"},
	}

	// Write ready file for claude round 2 (so file IPC succeeds)
	readyName := RoundSignalName("claude", 2, "ready")
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(sess.Dir(), readyName), []byte("1"), 0o644)
	}()

	// Also write a done + result file so collectRoundHookResults returns
	go func() {
		time.Sleep(200 * time.Millisecond)
		doneName := RoundSignalName("claude", 2, "done")
		resultName := RoundSignalName("claude", 2, "result.json")
		_ = os.WriteFile(filepath.Join(sess.Dir(), doneName), []byte("1"), 0o644)
		_ = os.WriteFile(filepath.Join(sess.Dir(), resultName), []byte(`{"output":"file ipc response","exit_code":0}`), 0o644)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	responses := executeRound(ctx, cfg, panes, sess, 2, prevResponses)

	// Then: response was collected via hook (file IPC path)
	require.Len(t, responses, 1)
	assert.Equal(t, "file ipc response", responses[0].Output)

	// Then: input file was created via file IPC (not SendLongText)
	inputName := RoundSignalName("claude", 2, "input.json")
	inputData, err := os.ReadFile(filepath.Join(sess.Dir(), inputName))
	require.NoError(t, err, "file IPC input file must exist")
	var input HookInput
	require.NoError(t, json.Unmarshal(inputData, &input))
	assert.Contains(t, input.Prompt, responseBeginMarker,
		"file IPC must carry the same response-file fallback contract as direct pane input")
	assert.NotEmpty(t, panes[0].responseFile)
}

func TestExecuteRound_FileIPCFallbackPersistsRoundCursor(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	session, err := NewHookSession("test-exec-round-cursor-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	session.SetHookProviders(map[string]bool{"claude": true})

	inputName := RoundSignalName("claude", 2, "input.json")
	require.NoError(t, os.Mkdir(filepath.Join(session.Dir(), inputName+".tmp"), 0o700),
		"the input temp-path collision forces direct pane fallback")
	require.NoError(t, os.WriteFile(
		filepath.Join(session.Dir(), RoundSignalName("claude", 2, "ready")), nil, 0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.Dir(), RoundSignalName("claude", 2, "result.json")),
		[]byte(`{"output":"direct fallback response","exit_code":0}`), 0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.Dir(), RoundSignalName("claude", 2, "done")), nil, 0o600,
	))
	consumed := consumeFileIPCAbort(session, "claude", 2, nil, nil)

	provider := ProviderConfig{Name: "claude", Binary: "echo", InteractiveInput: "stdin"}
	panes := []paneInfo{{provider: provider, paneID: "pane-1"}}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{provider}, Strategy: StrategyDebate,
		Prompt: "round 2 fallback", TimeoutSeconds: 5, Terminal: mock,
		Interactive: true, HookMode: true, InitialDelay: time.Millisecond,
	}

	responses := executeRound(
		context.Background(), cfg, panes, session, 2,
		[]ProviderResponse{{Provider: "claude", Output: "round 1"}},
	)
	require.NoError(t, <-consumed)

	require.Len(t, responses, 1)
	assert.Equal(t, "direct fallback response", responses[0].Output)
	assert.NotEmpty(t, mock.sendLongTextCalls, "failed file IPC must fall back to pane input")
	cursor, err := os.ReadFile(filepath.Join(session.Dir(), "claude-round-cursor"))
	require.NoError(t, err)
	assert.Equal(t, "2", string(cursor))
}

// TestWaitForDoneRoundCtx_ZeroRound_FallsBack verifies WaitForDoneRoundCtx
// falls back to WaitForDone for round=0 (covering the else branch).
func TestWaitForDoneRoundCtx_ZeroRound_FallsBack(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-donectx-zero-round")
	require.NoError(t, err)
	defer sess.Cleanup()

	// Given: provider-done file appears
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(sess.Dir(), "claude-done"), []byte("1"), 0o644)
	}()

	// When: WaitForDoneRoundCtx with round=0
	err = sess.WaitForDoneRoundCtx(context.Background(), 2*time.Second, "claude", 0)

	// Then: succeeds via WaitForDone fallback
	assert.NoError(t, err)
}

// TestWaitForReadyCtx_ContextCancelled verifies WaitForReadyCtx returns
// error when context is cancelled before ready file appears.
func TestWaitForReadyCtx_ContextCancelled(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-ready-ctx-cancel")
	require.NoError(t, err)
	defer sess.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err = sess.WaitForReadyCtx(ctx, 5*time.Second, "claude", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}
