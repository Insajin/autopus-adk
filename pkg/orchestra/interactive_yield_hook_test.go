package orchestra

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type yieldHookTerminal struct {
	mockTerminal
}

func (t *yieldHookTerminal) SplitPane(_ context.Context, dir terminal.Direction) (terminal.PaneID, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.splitPaneCalls = append(t.splitPaneCalls, dir)
	t.nextPaneID++
	id := terminal.PaneID("surface:1")
	t.createdPanes = append(t.createdPanes, id)
	return id, nil
}

func (t *yieldHookTerminal) WorkspaceRef() (string, error) {
	return "workspace:1", nil
}

func (t *yieldHookTerminal) WithWorkspaceRef(string) (terminal.Terminal, error) {
	return t, nil
}

func TestReleaseYieldHookWaiters_WritesAllAbortsAndWaitsForConsumption(t *testing.T) {
	session, err := NewHookSession("yield-release-success-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	session.SetHookProviders(map[string]bool{"claude": true, "codex": true})
	panes := []paneInfo{
		{provider: ProviderConfig{Name: "claude"}},
		{provider: ProviderConfig{Name: "codex"}},
	}
	for _, provider := range []string{"claude", "codex"} {
		require.NoError(t, session.writeArtifact(RoundSignalName(provider, 2, "ready"), nil, 0o600))
	}

	consumed := consumeYieldAborts(session, []string{"claude", "codex"}, 2)
	err = releaseYieldHookWaiters(context.Background(), session, panes, 2, time.Second)

	require.NoError(t, err)
	require.NoError(t, <-consumed)
	for _, provider := range []string{"claude", "codex"} {
		_, abortErr := session.statArtifact(RoundSignalName(provider, 2, "abort"))
		assert.True(t, errors.Is(abortErr, os.ErrNotExist))
	}
}

func TestReleaseYieldHookWaiters_Failures(t *testing.T) {
	t.Run("write", func(t *testing.T) {
		session, err := NewHookSession("yield-release-write-" + NewSessionID())
		require.NoError(t, err)
		defer session.Cleanup()
		session.SetHookProviders(map[string]bool{"../unsafe": true})

		err = releaseYieldHookWaiters(context.Background(), session, []paneInfo{{
			provider: ProviderConfig{Name: "../unsafe"},
		}}, 2, time.Second)

		assert.ErrorContains(t, err, "write abort")
	})

	t.Run("timeout", func(t *testing.T) {
		session, err := NewHookSession("yield-release-timeout-" + NewSessionID())
		require.NoError(t, err)
		defer session.Cleanup()
		session.SetHookProviders(map[string]bool{"claude": true})

		err = releaseYieldHookWaiters(context.Background(), session, []paneInfo{{
			provider: ProviderConfig{Name: "claude"},
		}}, 2, 40*time.Millisecond)

		assert.ErrorContains(t, err, "wait for hook abort consumption")
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("context", func(t *testing.T) {
		session, err := NewHookSession("yield-release-context-" + NewSessionID())
		require.NoError(t, err)
		defer session.Cleanup()
		session.SetHookProviders(map[string]bool{"claude": true})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = releaseYieldHookWaiters(ctx, session, []paneInfo{{
			provider: ProviderConfig{Name: "claude"},
		}}, 2, time.Second)

		assert.ErrorIs(t, err, context.Canceled)
		_, statErr := session.statArtifact(RoundSignalName("claude", 2, "abort"))
		assert.True(t, errors.Is(statErr, os.ErrNotExist))
	})
}

func TestReleaseYieldHookWaiters_IgnoresHooklessAndSkippedPanes(t *testing.T) {
	session, err := NewHookSession("yield-release-ignore-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	session.SetHookProviders(map[string]bool{"claude": true})

	err = releaseYieldHookWaiters(context.Background(), session, []paneInfo{
		{provider: ProviderConfig{Name: "claude"}, skipWait: true},
		{provider: ProviderConfig{Name: "opencode"}},
	}, 2, 40*time.Millisecond)

	require.NoError(t, err)
	_, statErr := session.statArtifact(RoundSignalName("claude", 2, "abort"))
	assert.True(t, errors.Is(statErr, os.ErrNotExist))
}

func TestRunPaneDebate_YieldReleasesHookBeforeSavingSession(t *testing.T) {
	isolateSurfaceTracker(t)
	t.Setenv("TMPDIR", t.TempDir())
	term := &yieldHookTerminal{mockTerminal: mockTerminal{name: "cmux"}}
	cfg := yieldHookDebateConfig(t, term, "yield-release-order")
	beforeSave := make(chan bool, 1)
	producer := publishYieldRoundOne(cfg.SessionID, true, beforeSave)

	result, err := runPaneDebate(context.Background(), cfg, 2, 700*time.Millisecond, time.Now())

	require.NoError(t, err)
	require.NoError(t, <-producer)
	assert.True(t, <-beforeSave, "abort must be consumed before SaveSession creates its directory")
	require.NotNil(t, result.Yield)
	t.Cleanup(func() { _ = RemoveSession(result.Yield.SessionID) })
	_, loadErr := LoadSession(result.Yield.SessionID)
	require.NoError(t, loadErr)
	assert.Empty(t, term.closeCalls, "durable yield owns the pane")
}

func TestRunPaneDebate_YieldReleaseFailureKeepsPaneOwnership(t *testing.T) {
	isolateSurfaceTracker(t)
	t.Setenv("TMPDIR", t.TempDir())
	term := &yieldHookTerminal{mockTerminal: mockTerminal{name: "cmux"}}
	cfg := yieldHookDebateConfig(t, term, "yield-release-failure")
	producer := publishYieldRoundOne(cfg.SessionID, false, nil)

	result, err := runPaneDebate(context.Background(), cfg, 2, 500*time.Millisecond, time.Now())

	require.Error(t, err)
	require.NoError(t, <-producer)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "release hook waiters before yield")
	assert.Equal(t, []string{"surface:1"}, term.closeCalls)
	_, statErr := os.Stat(sessionDirectoryPath())
	assert.True(t, errors.Is(statErr, os.ErrNotExist), "release failure must not create a yield session")
}

func yieldHookDebateConfig(t *testing.T, term terminal.Terminal, sessionID string) OrchestraConfig {
	t.Helper()
	provider := echoProvider("claude")
	provider.InteractiveInput = "args"
	provider.PromptViaArgs = true
	return OrchestraConfig{
		Providers: []ProviderConfig{provider}, Strategy: StrategyDebate,
		Prompt: "yield after a hook round", TimeoutSeconds: 1,
		Terminal: term, Interactive: true, HookMode: true, SessionID: sessionID,
		YieldRounds: true, NoJudge: true, InitialDelay: time.Millisecond,
		WorkingDir: t.TempDir(),
	}
}

func publishYieldRoundOne(sessionID string, consumeAbort bool, beforeSave chan<- bool) <-chan error {
	done := make(chan error, 1)
	go func() {
		dir := filepath.Join(os.TempDir(), hookBaseDirectoryName, sessionID)
		if err := waitForPath(filepath.Join(dir, "."), time.Second); err != nil {
			done <- err
			return
		}
		if err := os.WriteFile(filepath.Join(dir, RoundSignalName("claude", 1, "result.json")), []byte(`{"output":"round one","exit_code":0}`), 0o600); err != nil {
			done <- err
			return
		}
		if err := os.WriteFile(filepath.Join(dir, RoundSignalName("claude", 1, "done")), nil, 0o600); err != nil {
			done <- err
			return
		}
		ready := filepath.Join(dir, RoundSignalName("claude", 2, "ready"))
		if err := os.WriteFile(ready, nil, 0o600); err != nil {
			done <- err
			return
		}
		if consumeAbort {
			abort := filepath.Join(dir, RoundSignalName("claude", 2, "abort"))
			if err := waitForPath(abort, 2*time.Second); err != nil {
				done <- err
				return
			}
			if beforeSave != nil {
				_, err := os.Stat(sessionDirectoryPath())
				beforeSave <- errors.Is(err, os.ErrNotExist)
			}
			_ = os.Remove(ready)
			_ = os.Remove(abort)
		}
		done <- nil
	}()
	return done
}

func consumeYieldAborts(session *HookSession, providers []string, round int) <-chan error {
	done := make(chan error, 1)
	go func() {
		for _, provider := range providers {
			if err := waitForPath(filepath.Join(session.Dir(), RoundSignalName(provider, round, "abort")), time.Second); err != nil {
				done <- err
				return
			}
		}
		for _, provider := range providers {
			_ = session.removeArtifact(RoundSignalName(provider, round, "ready"))
			_ = session.removeArtifact(RoundSignalName(provider, round, "abort"))
		}
		done <- nil
	}()
	return done
}

func waitForPath(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
	return context.DeadlineExceeded
}
