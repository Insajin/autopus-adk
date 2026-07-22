package orchestra

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const codexStablePrompt = "› Summarize recent commits\n"

func TestWaitForPaneReady_CodexMissingArtifactDeactivatesStartupHook(t *testing.T) {
	session, err := NewHookSession("codex-startup-inactive-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()

	term := &seqScreenMock{name: "cmux", screens: []string{codexStablePrompt}}
	ready := waitForPaneReady(
		context.Background(), term, "surface:codex-untrusted",
		SessionReadyPatterns(), 3*time.Second, session, "CODEX", 0,
	)

	assert.True(t, ready, "a stable Codex prompt must survive an untrusted project startup hook")
	assert.False(t, session.HasStartupHook("codex"),
		"the missing Codex startup artifact must disable only startup trust for this run")
	assert.True(t, session.HasHook("codex"),
		"startup trust fallback must not disable the independent completion hook")
	assert.GreaterOrEqual(t, term.readCalls, 2)
}

func TestWaitForPaneReady_NonCodexMissingArtifactRemainsFailClosed(t *testing.T) {
	on := true
	tests := []struct {
		name     string
		provider ProviderConfig
		screen   string
	}{
		{
			name: "claude",
			provider: ProviderConfig{
				Name: "claude", HasStartupHook: &on,
			},
			screen: "❯ Try \"write a test for <filepath>\"\n",
		},
		{
			name: "gemini explicit startup",
			provider: ProviderConfig{
				Name: "gemini", HasStartupHook: &on,
			},
			screen: "> Type your message\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := NewHookSession("non-codex-startup-" + sanitizeProviderName(tt.name) + "-" + NewSessionID())
			require.NoError(t, err)
			defer session.Cleanup()
			session.ApplyProviderHooks([]ProviderConfig{tt.provider})
			term := &seqScreenMock{name: "cmux", screens: []string{tt.screen}}

			ready := waitForPaneReady(
				context.Background(), term, "surface:non-codex",
				SessionReadyPatterns(), 700*time.Millisecond,
				session, tt.provider.Name, 0,
			)

			assert.False(t, ready)
			assert.True(t, session.HasStartupHook(tt.provider.Name),
				"non-Codex startup contracts must remain fail-closed")
		})
	}
}

func TestWaitForPaneReady_CodexBlockerDoesNotDeactivateStartupHook(t *testing.T) {
	session, err := NewHookSession("codex-startup-blocker-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	term := &seqScreenMock{name: "cmux", screens: []string{
		codexStablePrompt + "PostCompact hooks\nTurn hooks on or off\nPress esc to go back\n",
	}}

	ready := waitForPaneReady(
		context.Background(), term, "surface:codex-blocked",
		SessionReadyPatterns(), 1300*time.Millisecond, session, "codex", 0,
	)

	assert.False(t, ready)
	assert.True(t, session.HasStartupHook("codex"),
		"a blocker screen is not evidence that only the startup hook is inactive")
}

func TestWaitForPaneReady_CodexWorkingScreenDoesNotDeactivateStartupHook(t *testing.T) {
	session, err := NewHookSession("codex-startup-working-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	term := &seqScreenMock{name: "cmux", screens: []string{
		codexStablePrompt + "• Working (2s • esc to interrupt)\n",
	}}

	ready := waitForPaneReady(
		context.Background(), term, "surface:codex-working",
		SessionReadyPatterns(), 1300*time.Millisecond, session, "codex", 0,
	)

	assert.False(t, ready)
	assert.True(t, session.HasStartupHook("codex"),
		"an active Codex TUI must not be mistaken for an inactive startup hook")
}

func TestRecreatePane_CodexStartupInactiveUsesStableDirectReadiness(t *testing.T) {
	term := newRecoveryLaunchTerminal()
	term.readScreenOutput = codexStablePrompt
	provider := ProviderConfig{
		Name: "codex", Binary: "codex", StartupTimeout: 3 * time.Second,
	}
	cfg, session := newRecoveryHookConfig(
		t, term, "recovery-codex-startup-inactive", provider,
	)

	ready := waitForPaneReady(
		context.Background(), term, "surface:initial-codex",
		SessionReadyPatterns(), provider.StartupTimeout, session, provider.Name, 0,
	)
	require.True(t, ready)
	require.False(t, session.HasStartupHook(provider.Name))

	old := paneInfo{paneID: "old-pane", provider: provider}
	got, err := recreatePane(context.Background(), cfg, old, 2)

	require.NoError(t, err)
	assert.Equal(t, terminal.PaneID("pane-1"), got.paneID)
	assert.Equal(t, 2, got.directPromptRound)
	assert.Contains(t, term.closeCalls, "old-pane")
	_, statErr := session.statArtifact(RoundSignalName(provider.Name, 2, "ready"))
	assert.ErrorIs(t, statErr, os.ErrNotExist,
		"recovery must not require a startup artifact after run-level Codex trust fallback")
}

func TestRecreatePane_CodexStartupInactiveRejectsWorkingScreen(t *testing.T) {
	term := newRecoveryLaunchTerminal()
	term.readScreenOutput = codexStablePrompt + "• Working (2s • esc to interrupt)\n"
	provider := ProviderConfig{
		Name: "codex", Binary: "codex", StartupTimeout: 700 * time.Millisecond,
	}
	cfg, session := newRecoveryHookConfig(
		t, term, "recovery-codex-startup-working", provider,
	)
	require.True(t, session.deactivateCodexStartupHook(provider.Name, 0))
	old := paneInfo{paneID: "old-pane", provider: provider}

	got, err := recreatePane(context.Background(), cfg, old, 2)

	require.Error(t, err)
	assert.Equal(t, old, got)
	assert.Contains(t, term.closeCalls, "pane-1")
	assert.NotContains(t, term.closeCalls, "old-pane")
}

func TestHookSession_CompletionAndStartupHealthAreIndependent(t *testing.T) {
	session, err := NewHookSession("hook-health-independent-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()

	provenance, err := session.deactivateContinuationHook(
		ProviderConfig{Name: "codex", Binary: "codex"}, 1,
	)

	require.NoError(t, err)
	assert.Equal(t, hookCompletionResponseFileOnly, provenance)
	assert.False(t, session.HasHook("codex"))
	assert.True(t, session.HasStartupHook("codex"),
		"completion fallback must not mutate startup trust")
}

func TestHookSession_StartupHealthIsConcurrentAndZeroValueSafe(t *testing.T) {
	var session HookSession
	assert.False(t, session.HasStartupHook("codex"))
	assert.False(t, session.deactivateCodexStartupHook("codex", 0))

	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(round int) {
			defer wg.Done()
			if round%3 == 0 {
				session.SetStartupHookProviders(map[string]bool{"CODEX": true})
				return
			}
			if round%3 == 1 {
				_ = session.HasStartupHook("codex")
				return
			}
			_ = session.deactivateCodexStartupHook("CODEX", round)
		}(i)
	}
	wg.Wait()

	session.SetStartupHookProviders(map[string]bool{"CODEX": true})
	require.True(t, session.HasStartupHook("codex"))
	require.True(t, session.deactivateCodexStartupHook("CODEX", 7))
	assert.False(t, session.HasStartupHook("codex"),
		"provider aliases must share one startup-health identity")
}
