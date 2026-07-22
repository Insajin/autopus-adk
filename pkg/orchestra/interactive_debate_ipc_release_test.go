package orchestra

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTryFileIPC_ReadyTimeoutLateWaiterAllowsSafeFallback(t *testing.T) {
	session, err := NewHookSession("ipc-late-waiter-" + NewSessionID())
	require.NoError(t, err)
	t.Cleanup(session.Cleanup)

	consumed := make(chan error, 1)
	go func() {
		abort := RoundSignalName("claude", 2, "abort")
		if err := waitForFileIPCArtifact(session, abort, time.Second); err != nil {
			consumed <- err
			return
		}
		ready := RoundSignalName("claude", 2, "ready")
		if err := session.writeArtifact(ready, nil, 0o600); err != nil {
			consumed <- err
			return
		}
		consumed <- removeFileIPCReleaseArtifacts(session, "claude", 2)
	}()

	outcome, ipcErr := tryFileIPCWithTimeouts(
		context.Background(), session, "claude", 2, "prompt", 30*time.Millisecond, time.Second,
	)

	assert.Equal(t, fileIPCSafeFallback, outcome)
	assert.ErrorContains(t, ipcErr, "wait for ready")
	require.NoError(t, <-consumed)
}

func TestTryFileIPC_AcknowledgementTimeoutIsReleaseFailure(t *testing.T) {
	session, err := NewHookSession("ipc-ack-timeout-" + NewSessionID())
	require.NoError(t, err)
	t.Cleanup(session.Cleanup)
	require.NoError(t, session.writeArtifact(RoundSignalName("claude", 2, "ready"), nil, 0o600))
	require.NoError(t, os.Mkdir(
		filepath.Join(session.Dir(), RoundSignalName("claude", 2, "input.json")+".tmp"), 0o700,
	))

	outcome, ipcErr := tryFileIPCWithTimeouts(
		context.Background(), session, "claude", 2, "prompt", time.Second, 40*time.Millisecond,
	)

	assert.Equal(t, fileIPCReleaseFailure, outcome)
	assert.ErrorIs(t, ipcErr, context.DeadlineExceeded)
	assert.ErrorContains(t, ipcErr, "abort consumption")
}

func TestExecuteRound_FileIPCFallbackWaitsForAbortAcknowledgement(t *testing.T) {
	fixture := newFileIPCReleaseRoundFixture(t, false)
	published := make(chan struct{})
	release := make(chan struct{})
	consumed := consumeFileIPCAbort(fixture.session, "claude", 2, published, release)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	responses := make(chan []ProviderResponse, 1)
	go func() {
		responses <- executeRound(ctx, fixture.cfg, fixture.panes, fixture.session, 2, fixture.previous)
	}()

	select {
	case <-published:
	case <-ctx.Done():
		t.Fatal("abort was not published")
	}
	time.Sleep(350 * time.Millisecond)
	longCalls, commandCalls := fileIPCDirectCallCounts(fixture.terminal)
	assert.Zero(t, longCalls+commandCalls, "direct input must wait for abort acknowledgement")
	close(release)
	require.NoError(t, <-consumed)

	select {
	case got := <-responses:
		require.Len(t, got, 1)
		assert.Equal(t, "fallback response", got[0].Output)
	case <-ctx.Done():
		t.Fatal("round did not resume after abort acknowledgement")
	}
	longCalls, _ = fileIPCDirectCallCounts(fixture.terminal)
	assert.Positive(t, longCalls)
	assert.Contains(t, fileIPCPromptFailure(fixture.store), "write input")
}

func TestExecuteRound_AbortWriteFailureSkipsDirectInput(t *testing.T) {
	fixture := newFileIPCReleaseRoundFixture(t, true)

	responses := executeRound(
		context.Background(), fixture.cfg, fixture.panes, fixture.session, 2, fixture.previous,
	)

	require.Len(t, responses, 1)
	assert.Equal(t, skippedHookCollectionError, responses[0].Error)
	longCalls, commandCalls := fileIPCDirectCallCounts(fixture.terminal)
	assert.Zero(t, longCalls+commandCalls)
	assert.Contains(t, fileIPCPromptFailure(fixture.store), "write abort")
}

type fileIPCReleaseRoundFixture struct {
	session  *HookSession
	terminal *mockTerminal
	store    *reliabilityStore
	cfg      OrchestraConfig
	panes    []paneInfo
	previous []ProviderResponse
}

func newFileIPCReleaseRoundFixture(t *testing.T, blockAbort bool) fileIPCReleaseRoundFixture {
	t.Helper()
	session, err := NewHookSession("ipc-release-round-" + NewSessionID())
	require.NoError(t, err)
	t.Cleanup(session.Cleanup)
	session.SetHookProviders(map[string]bool{"claude": true})
	require.NoError(t, session.writeArtifact(RoundSignalName("claude", 2, "ready"), nil, 0o600))
	require.NoError(t, os.Mkdir(
		filepath.Join(session.Dir(), RoundSignalName("claude", 2, "input.json")+".tmp"), 0o700,
	))
	if blockAbort {
		require.NoError(t, os.Mkdir(
			filepath.Join(session.Dir(), RoundSignalName("claude", 2, "abort")), 0o700,
		))
	} else {
		require.NoError(t, session.writeJSONArtifact(
			RoundSignalName("claude", 2, "result.json"), HookResult{Output: "fallback response"},
		))
		require.NoError(t, session.writeArtifact(RoundSignalName("claude", 2, "done"), nil, 0o600))
	}
	provider := ProviderConfig{Name: "claude", Binary: "echo", InteractiveInput: "stdin"}
	terminal := newCmuxMock()
	terminal.readScreenOutput = "❯\n"
	store := &reliabilityStore{runID: "ipc-release", dir: t.TempDir()}
	return fileIPCReleaseRoundFixture{
		session: session, terminal: terminal, store: store,
		cfg: OrchestraConfig{
			Providers: []ProviderConfig{provider}, Strategy: StrategyDebate, Prompt: "fallback",
			TimeoutSeconds: 1, Terminal: terminal, Interactive: true, HookMode: true,
			InitialDelay: time.Millisecond, RunID: "ipc-release", ReliabilityStore: store,
		},
		panes:    []paneInfo{{provider: provider, paneID: "pane-1"}},
		previous: []ProviderResponse{{Provider: "codex", Output: "round one"}},
	}
}

func consumeFileIPCAbort(
	session *HookSession,
	provider string,
	round int,
	published chan<- struct{},
	release <-chan struct{},
) <-chan error {
	done := make(chan error, 1)
	go func() {
		if err := waitForFileIPCArtifact(session, RoundSignalName(provider, round, "abort"), time.Second); err != nil {
			done <- err
			return
		}
		if published != nil {
			close(published)
		}
		if release != nil {
			<-release
		}
		done <- removeFileIPCReleaseArtifacts(session, provider, round)
	}()
	return done
}

func removeFileIPCReleaseArtifacts(session *HookSession, provider string, round int) error {
	for _, suffix := range []string{"ready", "abort"} {
		if err := session.removeArtifact(RoundSignalName(provider, round, suffix)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func waitForFileIPCArtifact(session *HookSession, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := session.statArtifact(name); err == nil {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
	return context.DeadlineExceeded
}

func fileIPCDirectCallCounts(terminal *mockTerminal) (int, int) {
	terminal.mu.Lock()
	defer terminal.mu.Unlock()
	return len(terminal.sendLongTextCalls), len(terminal.sendCommandCalls)
}

func fileIPCPromptFailure(store *reliabilityStore) string {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, receipt := range store.prompt {
		if receipt.TransportMode == "file_ipc" && receipt.Status == "failed" {
			return receipt.Mismatch
		}
	}
	return ""
}
