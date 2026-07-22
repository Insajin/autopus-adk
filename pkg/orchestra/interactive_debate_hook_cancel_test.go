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

func TestCollectRoundHookResults_PreCancelledPreservesOwnedProviderCardinality(t *testing.T) {
	session := newTestHookSession(t)
	session.SetHookProviders(map[string]bool{"claude": true, "gemini": true})
	store := &reliabilityStore{runID: "pre-cancel", dir: t.TempDir()}
	providers := []ProviderConfig{
		{Name: "claude", ExecutionTimeout: time.Second},
		{Name: "opencode", ExecutionTimeout: time.Second},
		{Name: "custom-skip", ExecutionTimeout: time.Second},
		{Name: "gemini", ExecutionTimeout: time.Second},
	}
	panes := []paneInfo{
		{provider: providers[0]},
		{provider: providers[1]},
		{provider: providers[2], skipWait: true},
		{provider: providers[3]},
	}
	cfg := OrchestraConfig{Providers: providers, Strategy: StrategyDebate, RunID: "pre-cancel", ReliabilityStore: store}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	responses := collectRoundHookResults(ctx, cfg, session, 1, panes)
	elapsed := time.Since(started)

	byProvider := responsesByProvider(responses)
	require.Len(t, responses, 3, "two hook providers plus the skipped pane belong to this collector")
	assert.NotContains(t, byProvider, "opencode", "active hookless panes are collected by the poll path")
	assertCancelledHookResponse(t, byProvider["claude"])
	assertCancelledHookResponse(t, byProvider["gemini"])
	assert.Equal(t, skippedHookCollectionError, byProvider["custom-skip"].Error)
	assert.Less(t, elapsed, 250*time.Millisecond)
	require.Len(t, store.collection, 2, "each cancelled hook provider needs collection evidence")
	require.Len(t, store.events, 2, "each cancelled hook provider needs timeout evidence")

	combined := append(responses, ProviderResponse{
		Provider: "opencode", Output: "poll response", ExecutedBackend: paneBackendName,
	})
	result := buildDebateResult(
		OrchestraConfig{Providers: providers, Strategy: StrategyDebate},
		combined, [][]ProviderResponse{combined}, started,
	)
	assert.Len(t, result.Responses, 4)
	assert.Len(t, result.FailedProviders, 3)
	assert.Equal(t, 4, result.DispatchCount)
	assert.Len(t, result.RunReceipt.ProviderReceipts, 4)
}

func TestCollectRoundHookResults_MidCancelPreservesSuccessAndFailure(t *testing.T) {
	session := newTestHookSession(t)
	session.SetHookProviders(map[string]bool{"claude": true, "codex": true})
	store := &reliabilityStore{runID: "mid-cancel", dir: t.TempDir()}
	providers := []ProviderConfig{
		{Name: "claude", ExecutionTimeout: 2 * time.Second},
		{Name: "codex", ExecutionTimeout: 2 * time.Second},
	}
	writeRoundHookResult(t, session, "claude", 1, "completed before cancellation")
	ctx, cancel := context.WithCancel(context.Background())
	cancelled := make(chan struct{})
	go func() {
		time.Sleep(40 * time.Millisecond)
		cancel()
		close(cancelled)
	}()

	started := time.Now()
	responses := collectRoundHookResults(ctx, OrchestraConfig{
		Providers: providers, RunID: "mid-cancel", ReliabilityStore: store,
	}, session, 1, []paneInfo{{provider: providers[0]}, {provider: providers[1]}})
	elapsed := time.Since(started)
	<-cancelled

	byProvider := responsesByProvider(responses)
	require.Len(t, responses, 2)
	assert.Equal(t, "completed before cancellation", byProvider["claude"].Output)
	assert.False(t, byProvider["claude"].TimedOut)
	assertCancelledHookResponse(t, byProvider["codex"])
	assert.Less(t, elapsed, 250*time.Millisecond, "cancellation must unblock the waiting hook immediately")
	require.Len(t, store.collection, 2)
	require.Len(t, store.events, 1)
	assert.Equal(t, "codex", store.events[0].Correlation.ProviderID)
}

func assertCancelledHookResponse(t *testing.T, response ProviderResponse) {
	t.Helper()
	assert.NotEmpty(t, response.Provider)
	assert.True(t, response.TimedOut)
	assert.True(t, response.EmptyOutput)
	assert.Contains(t, response.Error, context.Canceled.Error())
	assert.NotEmpty(t, response.Receipt)
	assert.Equal(t, paneBackendName, response.ExecutedBackend)
	assert.Equal(t, "participant", response.Role)
}

func responsesByProvider(responses []ProviderResponse) map[string]ProviderResponse {
	indexed := make(map[string]ProviderResponse, len(responses))
	for _, response := range responses {
		indexed[response.Provider] = response
	}
	return indexed
}

func writeRoundHookResult(t *testing.T, session *HookSession, provider string, round int, output string) {
	t.Helper()
	data, err := json.Marshal(HookResult{Output: output})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(session.Dir(), RoundSignalName(provider, round, "result.json")), data, 0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.Dir(), RoundSignalName(provider, round, "done")), nil, 0o600,
	))
}
