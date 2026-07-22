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

func TestWaitAndCollectResults_HookResultPrecedesScreenFallback(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("pane-hook-collect-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	require.NoError(t, os.WriteFile(filepath.Join(session.Dir(), "claude-result.json"),
		[]byte(`{"output":"hook output","exit_code":0}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(session.Dir(), "claude-done"), nil, 0o600))

	term := newCmuxMock()
	term.readScreenOutput = "screen fallback must not win\n❯\n"
	provider := ProviderConfig{Name: "claude"}
	panes := []paneInfo{{provider: provider, paneID: "pane-hook"}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	responses := waitAndCollectResults(ctx, OrchestraConfig{
		Providers: []ProviderConfig{provider}, Terminal: term, HookMode: true,
	}, panes, DefaultCompletionPatterns(), time.Now(), nil, session, 0)

	require.Len(t, responses, 1)
	assert.Equal(t, "hook output", responses[0].Output)
	assert.False(t, responses[0].TimedOut)
	assert.Zero(t, term.readScreenCalls)
}

func TestWaitAndCollectResults_ResultWithoutDone_RemainsTimedOut(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("pane-hook-orphan-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	require.NoError(t, os.WriteFile(filepath.Join(session.Dir(), "claude-result.json"),
		[]byte(`{"output":"orphan hook output","exit_code":0}`), 0o600))

	term := newCmuxMock()
	term.readScreenOutput = "bounded screen fallback"
	provider := ProviderConfig{Name: "claude"}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	started := time.Now()

	responses := waitAndCollectResults(ctx, OrchestraConfig{
		Providers: []ProviderConfig{provider}, Terminal: term, HookMode: true,
	}, []paneInfo{{provider: provider, paneID: "pane-orphan"}},
		DefaultCompletionPatterns(), started, nil, session, 0)

	require.Len(t, responses, 1)
	assert.True(t, responses[0].TimedOut, "a result artifact cannot replace the missing done signal")
	assert.Contains(t, responses[0].Error, "completion detector timed out")
	assert.Equal(t, "orphan hook output", responses[0].Output, "partial result evidence may be preserved on a timeout")
	assert.Zero(t, term.readScreenCalls, "available partial evidence must avoid an unbounded final screen read")
	assert.Less(t, time.Since(started), time.Second, "collector must remain bounded by the caller context")
}

func TestWaitAndCollectResults_DetectorFailureDoesNotPromoteHookResult(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("pane-hook-detector-error-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	require.NoError(t, os.WriteFile(filepath.Join(session.Dir(), "claude-result.json"),
		[]byte(`{"output":"stale hook output","exit_code":0}`), 0o600))

	term := newCmuxMock()
	term.readScreenOutput = "detector failure screen fallback"
	provider := ProviderConfig{Name: "claude"}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	responses := waitAndCollectResults(ctx, OrchestraConfig{
		Providers: []ProviderConfig{provider}, Terminal: term, HookMode: true,
		CompletionDetector: &stubCompletionDetector{err: errors.New("injected detector failure")},
	}, []paneInfo{{provider: provider, paneID: "pane-detector-error"}},
		DefaultCompletionPatterns(), time.Now(), nil, session, 0)

	require.Len(t, responses, 1)
	assert.True(t, responses[0].TimedOut)
	assert.Equal(t, "stale hook output", responses[0].Output)
	assert.Zero(t, term.readScreenCalls)
}
