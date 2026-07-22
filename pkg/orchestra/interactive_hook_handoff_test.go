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

func TestRunPaneDebate_DelayedNextReadyKeepsHookFileIPC(t *testing.T) {
	isolateSurfaceTracker(t)
	t.Setenv("TMPDIR", t.TempDir())
	term := newResponseOnlyHookTerminal()
	term.maxResponses = 1
	cfg := responseOnlyHookDebateConfig(t, term, false)
	producerDone := make(chan error, 1)
	term.afterResponse = func(writeNumber int) {
		if writeNumber != 1 {
			return
		}
		term.mu.Lock()
		term.readScreenOutput = "codex>\n• Running Stop hook (1s • esc to interrupt)\n"
		term.mu.Unlock()
		go publishDelayedRoundTwo(cfg.SessionID, term, producerDone)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	result, err := runPaneDebate(ctx, cfg, 2, 5*time.Second, time.Now())

	require.NoError(t, <-producerDone)
	require.NoError(t, err)
	require.Len(t, result.RoundHistory, 2)
	require.Len(t, result.RoundHistory[1], 1)
	assert.Equal(t, "delayed hook round 2", result.RoundHistory[1][0].Output)
	assert.Equal(t, 1, term.writes(), "round 2 must remain on file IPC")
}

func TestRunPaneDebate_DelayedNextReadyYieldsAfterAbortAck(t *testing.T) {
	isolateSurfaceTracker(t)
	t.Setenv("TMPDIR", t.TempDir())
	term := newResponseOnlyHookTerminal()
	term.maxResponses = 1
	cfg := responseOnlyHookDebateConfig(t, term, true)
	producerDone := make(chan error, 1)
	term.afterResponse = func(writeNumber int) {
		if writeNumber != 1 {
			return
		}
		term.mu.Lock()
		term.readScreenOutput = "codex>\n• Running Stop hook (1s • esc to interrupt)\n"
		term.mu.Unlock()
		go publishDelayedYieldReady(cfg.SessionID, term, producerDone)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	result, err := runPaneDebate(ctx, cfg, 2, 5*time.Second, time.Now())

	require.NoError(t, <-producerDone)
	require.NoError(t, err)
	require.NotNil(t, result.Yield)
	t.Cleanup(func() { _ = RemoveSession(result.Yield.SessionID) })
	assert.Equal(t, 1, term.writes())
}

func TestCollectRoundHookResults_PermanentWorkingHandoffIsUnusable(t *testing.T) {
	session, err := NewHookSession("working-handoff-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	provider := ProviderConfig{Name: "codex", ExecutionTimeout: 2 * time.Second}
	responsePath := filepath.Join(t.TempDir(), "response.md")
	writeMarkedResponse(t, responsePath, "response while Stop hook runs")
	term := newResponseOnlyHookTerminal()
	term.readScreenOutput = "codex>\n• Running Stop hook (1s • esc to interrupt)\n"
	store := &reliabilityStore{runID: "working-handoff", dir: t.TempDir()}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{provider}, Terminal: term, DebateRounds: 2,
		RunID: "working-handoff", ReliabilityStore: store,
	}

	responses := collectRoundHookResults(context.Background(), cfg, session, 1, []paneInfo{{
		provider: provider, paneID: "surface:1", responseFile: responsePath,
	}})

	require.Len(t, responses, 1)
	assert.True(t, responses[0].TimedOut)
	assert.True(t, responses[0].EmptyOutput)
	assert.Empty(t, responses[0].Output)
	assert.Contains(t, responses[0].Error, "completion handoff")
	assert.True(t, session.HasHook("codex"), "working evidence must fail closed")
	require.Len(t, store.collection, 1)
	assert.Equal(t, "timeout", store.collection[0].Status)
	require.Len(t, store.events, 1)
	assert.Equal(t, "hook_timeout", store.events[0].Kind)
	assert.FileExists(t, filepath.Join(store.dir, "failure-bundle.json"))
}

func TestResolveHookCompletionHandoff_GenericWorkingThenIdleDeactivatesHook(t *testing.T) {
	session, err := NewHookSession("working-idle-handoff-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	provider := ProviderConfig{Name: "codex", Binary: "codex"}
	term := newTransitionResponseTerminal(1)
	cfg := OrchestraConfig{Providers: []ProviderConfig{provider}, Terminal: term, DebateRounds: 2}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	provenance, err := resolveHookCompletionHandoff(
		ctx, cfg, session, provider, paneInfo{provider: provider, paneID: "surface:1"}, 1,
	)

	require.NoError(t, err)
	assert.Equal(t, hookCompletionResponseFileOnly, provenance)
	assert.False(t, session.HasHook("codex"))
	assert.GreaterOrEqual(t, term.transitionReadCount(), 3)
}

func TestRunPaneDebate_GenericWorkingThenIdleUsesDirectRoundTwo(t *testing.T) {
	isolateSurfaceTracker(t)
	t.Setenv("TMPDIR", t.TempDir())
	term := newTransitionResponseTerminal(0)
	term.afterResponse = func(writeNumber int) {
		if writeNumber == 1 {
			term.setWorkingReads(6)
		}
	}
	cfg := responseOnlyHookDebateConfig(t, term, false)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	result, err := runPaneDebate(ctx, cfg, 2, 5*time.Second, time.Now())

	require.NoError(t, err)
	require.Len(t, result.RoundHistory, 2)
	require.Len(t, result.RoundHistory[1], 1)
	assert.Equal(t, "response-only round 2", result.RoundHistory[1][0].Output)
	assert.Equal(t, 2, term.writes(), "round 2 must use direct input after stable idle")
}

type transitionResponseTerminal struct {
	*responseOnlyHookTerminal
	workingReads    int
	transitionReads int
}

func newTransitionResponseTerminal(workingReads int) *transitionResponseTerminal {
	return &transitionResponseTerminal{
		responseOnlyHookTerminal: newResponseOnlyHookTerminal(),
		workingReads:             workingReads,
	}
}

func (t *transitionResponseTerminal) ReadScreen(_ context.Context, _ terminal.PaneID, _ terminal.ReadScreenOpts) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.transitionReads++
	if t.workingReads != 0 {
		if t.workingReads > 0 {
			t.workingReads--
		}
		return "codex>\n• Preparing... (1s • esc to interrupt)\n", nil
	}
	return "codex>\n", nil
}

func (t *transitionResponseTerminal) setWorkingReads(reads int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.workingReads = reads
}

func (t *transitionResponseTerminal) transitionReadCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.transitionReads
}

func TestCollectRoundHookResults_FinalRoundDoesNotMutateHookHealth(t *testing.T) {
	session, err := NewHookSession("final-round-health-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	provider := ProviderConfig{Name: "codex", ExecutionTimeout: 2 * time.Second}
	responsePath := filepath.Join(t.TempDir(), "response.md")
	writeMarkedResponse(t, responsePath, "final response")
	term := newResponseOnlyHookTerminal()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{provider}, Terminal: term, DebateRounds: 1,
	}

	responses := collectRoundHookResults(context.Background(), cfg, session, 1, []paneInfo{{
		provider: provider, paneID: "surface:1", responseFile: responsePath,
	}})

	require.Len(t, responses, 1)
	assert.Equal(t, "final response", responses[0].Output)
	assert.True(t, session.HasHook("codex"))
	assert.Zero(t, term.readScreenCalls, "a final round needs no handoff probe")
}

func TestCompletionArtifactProvenance_NextReadyWinsOverCurrentDone(t *testing.T) {
	session, err := NewHookSession("handoff-priority-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	provider := ProviderConfig{Name: "codex", Binary: "codex"}
	require.NoError(t, session.writeArtifact(RoundSignalName("codex", 1, "done"), nil, 0o600))

	provenance, err := session.completionArtifactProvenance(provider, 1)
	require.NoError(t, err)
	assert.Equal(t, hookCompletionDone, provenance)
	require.NoError(t, session.writeArtifact(RoundSignalName("codex", 2, "ready"), nil, 0o600))

	provenance, err = session.completionArtifactProvenance(provider, 1)
	require.NoError(t, err)
	assert.Equal(t, hookCompletionNextRoundReady, provenance)
}

func publishDelayedRoundTwo(sessionID string, term *responseOnlyHookTerminal, done chan<- error) {
	dir := filepath.Join(os.TempDir(), hookBaseDirectoryName, sessionID)
	currentDone := filepath.Join(dir, RoundSignalName("codex", 1, "done"))
	if err := os.WriteFile(currentDone, nil, 0o600); err != nil {
		done <- err
		return
	}
	time.Sleep(1500 * time.Millisecond)
	ready := filepath.Join(dir, RoundSignalName("codex", 2, "ready"))
	if err := os.WriteFile(ready, nil, 0o600); err != nil {
		done <- err
		return
	}
	term.mu.Lock()
	term.readScreenOutput = "codex>\n"
	term.mu.Unlock()
	input := filepath.Join(dir, RoundSignalName("codex", 2, "input.json"))
	if err := waitForPath(input, 3*time.Second); err != nil {
		done <- err
		return
	}
	result := filepath.Join(dir, RoundSignalName("codex", 2, "result.json"))
	if err := os.WriteFile(result, []byte(`{"output":"delayed hook round 2","exit_code":0}`), 0o600); err != nil {
		done <- err
		return
	}
	donePath := filepath.Join(dir, RoundSignalName("codex", 2, "done"))
	done <- os.WriteFile(donePath, nil, 0o600)
}

func publishDelayedYieldReady(sessionID string, term *responseOnlyHookTerminal, done chan<- error) {
	dir := filepath.Join(os.TempDir(), hookBaseDirectoryName, sessionID)
	currentDone := filepath.Join(dir, RoundSignalName("codex", 1, "done"))
	if err := os.WriteFile(currentDone, nil, 0o600); err != nil {
		done <- err
		return
	}
	time.Sleep(1500 * time.Millisecond)
	ready := filepath.Join(dir, RoundSignalName("codex", 2, "ready"))
	abort := filepath.Join(dir, RoundSignalName("codex", 2, "abort"))
	if _, err := os.Stat(abort); err == nil {
		_ = os.Remove(abort)
		done <- assert.AnError
		return
	}
	if err := os.WriteFile(ready, nil, 0o600); err != nil {
		done <- err
		return
	}
	term.mu.Lock()
	term.readScreenOutput = "codex>\n"
	term.mu.Unlock()
	if err := waitForPath(abort, 3*time.Second); err != nil {
		done <- err
		return
	}
	if err := os.Remove(ready); err != nil {
		done <- err
		return
	}
	done <- os.Remove(abort)
}
