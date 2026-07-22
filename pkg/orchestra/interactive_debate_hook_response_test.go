package orchestra

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectRoundHookResults_ClaudeResponseFileCompletesWithoutDone(t *testing.T) {
	session, err := NewHookSession("debate-hook-claude-response")
	require.NoError(t, err)
	defer session.Cleanup()
	provider := ProviderConfig{Name: "claude", ExecutionTimeout: 500 * time.Millisecond}
	responsePath := filepath.Join(t.TempDir(), "claude-response.md")
	writeMarkedResponse(t, responsePath, "claude file response")
	panes := []paneInfo{{provider: provider, responseFile: responsePath}}

	started := time.Now()
	responses := collectRoundHookResults(
		context.Background(), OrchestraConfig{Providers: []ProviderConfig{provider}}, session, 1, panes,
	)

	require.Len(t, responses, 1)
	assert.Equal(t, "claude file response", responses[0].Output)
	assert.False(t, responses[0].TimedOut)
	assert.Less(t, time.Since(started), 250*time.Millisecond)
}

func TestCollectRoundHookResults_CodexResponseFileCompletesAfterGraceWithoutDone(t *testing.T) {
	session, err := NewHookSession("debate-hook-codex-response")
	require.NoError(t, err)
	defer session.Cleanup()
	provider := ProviderConfig{Name: "codex", ExecutionTimeout: 2 * time.Second}
	responsePath := filepath.Join(t.TempDir(), "codex-response.md")
	writeMarkedResponse(t, responsePath, "codex file response")
	panes := []paneInfo{{provider: provider, responseFile: responsePath}}

	started := time.Now()
	responses := collectRoundHookResults(
		context.Background(), OrchestraConfig{Providers: []ProviderConfig{provider}}, session, 1, panes,
	)
	elapsed := time.Since(started)

	require.Len(t, responses, 1)
	assert.Equal(t, "codex file response", responses[0].Output)
	assert.False(t, responses[0].TimedOut)
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond)
	assert.Less(t, elapsed, 1500*time.Millisecond)
}

func TestCollectRoundHookResults_ResponseFileWinsOverInvalidHookResult(t *testing.T) {
	for _, hookResult := range []struct {
		name string
		data []byte
	}{
		{name: "missing"},
		{name: "malformed", data: []byte("{bad json")},
	} {
		t.Run(hookResult.name, func(t *testing.T) {
			session, err := NewHookSession("debate-hook-response-priority-" + hookResult.name)
			require.NoError(t, err)
			defer session.Cleanup()
			provider := ProviderConfig{Name: "claude", ExecutionTimeout: time.Second}
			responsePath := filepath.Join(t.TempDir(), "response.md")
			writeMarkedResponse(t, responsePath, "authoritative response file")
			panes := []paneInfo{{provider: provider, responseFile: responsePath}}
			donePath := filepath.Join(session.Dir(), RoundSignalName("claude", 1, "done"))
			require.NoError(t, os.WriteFile(donePath, []byte("1"), 0o600))
			if hookResult.data != nil {
				resultPath := filepath.Join(session.Dir(), RoundSignalName("claude", 1, "result.json"))
				require.NoError(t, os.WriteFile(resultPath, hookResult.data, 0o600))
			}

			responses := collectRoundHookResults(
				context.Background(), OrchestraConfig{Providers: []ProviderConfig{provider}}, session, 1, panes,
			)

			require.Len(t, responses, 1)
			assert.Equal(t, "authoritative response file", responses[0].Output)
			assert.False(t, responses[0].TimedOut)
		})
	}
}

func TestCollectRoundHookResults_SkippedPaneReturnsUnavailableImmediately(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderConfig
	}{
		{name: "hook provider", provider: ProviderConfig{Name: "claude", ExecutionTimeout: 600 * time.Millisecond}},
		{name: "provider without hook", provider: ProviderConfig{Name: "custom", ExecutionTimeout: 600 * time.Millisecond}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := newTestHookSession(t)
			store := &reliabilityStore{runID: "skip-wait", dir: t.TempDir()}
			panes := []paneInfo{{provider: tt.provider, skipWait: true}}
			cfg := OrchestraConfig{
				Providers:        []ProviderConfig{tt.provider},
				ReliabilityStore: store,
				RunID:            "skip-wait",
			}

			started := time.Now()
			responses := collectRoundHookResults(context.Background(), cfg, session, 1, panes)
			elapsed := time.Since(started)

			require.Len(t, responses, 1)
			assert.True(t, responses[0].TimedOut)
			assert.True(t, responses[0].EmptyOutput)
			assert.Equal(t, "provider was skipped before hook completion collection", responses[0].Error)
			assert.Empty(t, responses[0].Receipt)
			assert.Less(t, elapsed, 200*time.Millisecond)
			assert.Empty(t, store.collection, "a skipped pane must not create a timeout receipt")
			assert.Empty(t, store.events, "a skipped pane must not create a timeout event")
			assert.NoFileExists(t, filepath.Join(store.dir, "failure-bundle.json"))
		})
	}
}

func TestCollectRoundHookResults_CodexResponseBeforeGraceKeepsTimeout(t *testing.T) {
	session := newTestHookSession(t)
	store := &reliabilityStore{runID: "codex-grace-timeout", dir: t.TempDir()}
	provider := ProviderConfig{Name: "codex", ExecutionTimeout: 300 * time.Millisecond}
	responsePath := filepath.Join(t.TempDir(), "codex-response.md")
	writeMarkedResponse(t, responsePath, "response written before hook completion")
	panes := []paneInfo{{provider: provider, responseFile: responsePath}}
	cfg := OrchestraConfig{
		Providers:        []ProviderConfig{provider},
		ReliabilityStore: store,
		RunID:            "codex-grace-timeout",
	}

	started := time.Now()
	responses := collectRoundHookResults(context.Background(), cfg, session, 1, panes)
	elapsed := time.Since(started)

	require.Len(t, responses, 1)
	assert.True(t, responses[0].TimedOut)
	assert.True(t, responses[0].EmptyOutput)
	assert.Empty(t, responses[0].Output, "an incomplete detector must not be salvaged by a response file")
	assert.Contains(t, responses[0].Error, "timeout")
	assert.GreaterOrEqual(t, elapsed, 250*time.Millisecond)
	assert.Less(t, elapsed, time.Second)
	require.Len(t, store.collection, 1)
	assert.Equal(t, "timeout", store.collection[0].Status)
	assert.Equal(t, "hook", store.collection[0].CollectionMode)
	require.Len(t, store.events, 1)
	assert.Equal(t, "hook_timeout", store.events[0].Kind)
}

func TestCollectRoundHookResults_DoneThenDelayedResponsePrefersResponseFile(t *testing.T) {
	session := newTestHookSession(t)
	provider := ProviderConfig{Name: "codex", ExecutionTimeout: time.Second}
	responsePath := filepath.Join(t.TempDir(), "codex-response.md")
	panes := []paneInfo{{provider: provider, responseFile: responsePath}}
	donePath := filepath.Join(session.Dir(), RoundSignalName("codex", 1, "done"))
	require.NoError(t, os.WriteFile(donePath, []byte("1"), 0o600))
	writeErr := make(chan error, 1)
	go func() {
		time.Sleep(75 * time.Millisecond)
		writeErr <- os.WriteFile(responsePath, []byte(markedResponse("response after done")), 0o600)
	}()

	started := time.Now()
	responses := collectRoundHookResults(
		context.Background(), OrchestraConfig{Providers: []ProviderConfig{provider}}, session, 1, panes,
	)
	elapsed := time.Since(started)
	require.NoError(t, <-writeErr)

	require.Len(t, responses, 1)
	assert.Equal(t, "response after done", responses[0].Output)
	assert.False(t, responses[0].TimedOut)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
	assert.Less(t, elapsed, 500*time.Millisecond)
}

func TestCollectRoundHookResults_DoneResponseSettleRespectsContext(t *testing.T) {
	session := newTestHookSession(t)
	provider := ProviderConfig{Name: "codex", ExecutionTimeout: time.Second}
	responsePath := filepath.Join(t.TempDir(), "missing-response.md")
	panes := []paneInfo{{provider: provider, responseFile: responsePath}}
	donePath := filepath.Join(session.Dir(), RoundSignalName("codex", 1, "done"))
	require.NoError(t, os.WriteFile(donePath, []byte("1"), 0o600))
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	started := time.Now()
	responses := collectRoundHookResults(ctx, OrchestraConfig{Providers: []ProviderConfig{provider}}, session, 1, panes)
	elapsed := time.Since(started)

	require.Len(t, responses, 1)
	assert.Empty(t, responses[0].Output)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
	assert.Less(t, elapsed, 250*time.Millisecond)
}

func TestCollectRoundHookResults_ResponseThenDelayedDoneKeepsEarlyDoneContract(t *testing.T) {
	session := newTestHookSession(t)
	provider := ProviderConfig{Name: "codex", ExecutionTimeout: 2 * time.Second}
	responsePath := filepath.Join(t.TempDir(), "codex-response.md")
	writeMarkedResponse(t, responsePath, "response before done")
	panes := []paneInfo{{provider: provider, responseFile: responsePath}}
	writeErr := make(chan error, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		donePath := filepath.Join(session.Dir(), RoundSignalName("codex", 1, "done"))
		writeErr <- os.WriteFile(donePath, []byte("1"), 0o600)
	}()

	started := time.Now()
	responses := collectRoundHookResults(
		context.Background(), OrchestraConfig{Providers: []ProviderConfig{provider}}, session, 1, panes,
	)
	elapsed := time.Since(started)
	require.NoError(t, <-writeErr)

	require.Len(t, responses, 1)
	assert.Equal(t, "response before done", responses[0].Output)
	assert.False(t, responses[0].TimedOut)
	assert.GreaterOrEqual(t, elapsed, 250*time.Millisecond)
	assert.Less(t, elapsed, 900*time.Millisecond)
}
