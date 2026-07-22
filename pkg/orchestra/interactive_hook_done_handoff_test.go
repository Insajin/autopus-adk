package orchestra

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPaneDebate_DoneOnlyStableIdleFallsBackToDirectPolling(t *testing.T) {
	isolateSurfaceTracker(t)
	t.Setenv("TMPDIR", t.TempDir())
	term := newResponseOnlyHookTerminal()
	cfg := responseOnlyHookDebateConfig(t, term, false)
	doneWritten := publishDoneAfterFirstResponse(term, cfg.SessionID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := runPaneDebate(ctx, cfg, 2, 5*time.Second, time.Now())

	require.NoError(t, <-doneWritten)
	require.NoError(t, err)
	require.Len(t, result.RoundHistory, 2)
	require.Len(t, result.RoundHistory[1], 1)
	assert.Equal(t, "response-only round 2", result.RoundHistory[1][0].Output)
	assert.Equal(t, 2, term.writes(), "done without next-ready must fall back to direct input")
}

func TestRunPaneDebate_DoneOnlyStableIdleYieldsWithoutAbortWaiter(t *testing.T) {
	isolateSurfaceTracker(t)
	t.Setenv("TMPDIR", t.TempDir())
	term := newResponseOnlyHookTerminal()
	cfg := responseOnlyHookDebateConfig(t, term, true)
	doneWritten := publishDoneAfterFirstResponse(term, cfg.SessionID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := runPaneDebate(ctx, cfg, 2, 5*time.Second, time.Now())

	require.NoError(t, <-doneWritten)
	require.NoError(t, err)
	require.NotNil(t, result.Yield)
	t.Cleanup(func() { _ = RemoveSession(result.Yield.SessionID) })
	assert.Equal(t, 1, term.writes())
}

func publishDoneAfterFirstResponse(term *responseOnlyHookTerminal, sessionID string) <-chan error {
	done := make(chan error, 1)
	term.afterResponse = func(writeNumber int) {
		if writeNumber != 1 {
			return
		}
		path := filepath.Join(os.TempDir(), hookBaseDirectoryName, sessionID,
			RoundSignalName("codex", 1, "done"))
		done <- os.WriteFile(path, nil, 0o600)
	}
	return done
}

type readyDuringStableFrameTerminal struct {
	mockTerminal
	session *HookSession
	reads   int
}

func (t *readyDuringStableFrameTerminal) ReadScreen(_ context.Context, _ terminal.PaneID, _ terminal.ReadScreenOpts) (string, error) {
	t.mu.Lock()
	t.reads++
	reads := t.reads
	t.mu.Unlock()
	if reads == hookStableIdleFrames {
		if err := t.session.writeArtifact(RoundSignalName("codex", 2, "ready"), nil, 0o600); err != nil {
			return "", err
		}
	}
	return "codex>\n", nil
}

func TestResolveHookCompletionHandoff_ReadyWinsFinalIdleRace(t *testing.T) {
	session, err := NewHookSession("handoff-ready-race-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	require.NoError(t, session.writeArtifact(RoundSignalName("codex", 1, "done"), nil, 0o600))
	term := &readyDuringStableFrameTerminal{
		mockTerminal: mockTerminal{name: "cmux"}, session: session,
	}
	provider := ProviderConfig{Name: "codex", Binary: "codex"}
	cfg := OrchestraConfig{Providers: []ProviderConfig{provider}, Terminal: term, DebateRounds: 2}

	provenance, err := resolveHookCompletionHandoff(
		context.Background(), cfg, session, provider,
		paneInfo{provider: provider, paneID: "surface:1"}, 1,
	)

	require.NoError(t, err)
	assert.Equal(t, hookCompletionNextRoundReady, provenance)
	assert.True(t, session.HasHook("codex"))
}
