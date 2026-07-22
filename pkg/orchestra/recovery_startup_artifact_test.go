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

func newRecoveryHookConfig(
	t *testing.T,
	term terminal.Terminal,
	sessionID string,
	providers ...ProviderConfig,
) (OrchestraConfig, *HookSession) {
	t.Helper()
	session, err := NewHookSession(sessionID)
	require.NoError(t, err)
	t.Cleanup(session.Cleanup)
	session.ApplyProviderHooks(providers)
	manager := NewSurfaceManager(term)
	manager.setHookSession(session)
	return OrchestraConfig{
		Terminal: term, HookMode: true, SessionID: sessionID, SurfaceMgr: manager,
	}, session
}

func TestRecreatePane_StaleStartupArtifactCannotCommitReplacement(t *testing.T) {
	term := newRecoveryLaunchTerminal()
	provider := ProviderConfig{
		Name: "claude-code", Binary: "claude", StartupTimeout: 450 * time.Millisecond,
	}
	cfg, session := newRecoveryHookConfig(t, term, "recovery-stale-ready", provider)
	require.NoError(t, session.writeArtifact("claude-round2-ready", nil, 0o600))
	old := paneInfo{paneID: "old-pane", provider: provider}

	got, err := recreatePane(context.Background(), cfg, old, 2)

	require.Error(t, err)
	assert.Equal(t, old, got)
	assert.Contains(t, term.closeCalls, "pane-1")
	assert.NotContains(t, term.closeCalls, "old-pane")
}

func TestRecreatePane_FreshStartupArtifactAndStableFramesCommit(t *testing.T) {
	term := newRecoveryLaunchTerminal()
	provider := ProviderConfig{
		Name: "claude-code", Binary: "claude", StartupTimeout: time.Second,
	}
	cfg, session := newRecoveryHookConfig(t, term, "recovery-fresh-ready", provider)
	readyName := "claude-round2-ready"
	require.NoError(t, session.writeArtifact(readyName, nil, 0o600))
	staleWasReset := false
	term.onLaunch = func(terminal.PaneID) {
		_, staleErr := session.statArtifact(readyName)
		staleWasReset = os.IsNotExist(staleErr)
		require.NoError(t, session.writeArtifact(readyName, nil, 0o600))
	}
	old := paneInfo{paneID: "old-pane", provider: provider}

	got, err := recreatePane(context.Background(), cfg, old, 2)

	require.NoError(t, err)
	assert.True(t, staleWasReset, "stale ready must be removed before provider launch")
	assert.Equal(t, terminal.PaneID("pane-1"), got.paneID)
	assert.Equal(t, 2, got.directPromptRound)
	assert.Contains(t, term.closeCalls, "old-pane")
	_, statErr := session.statArtifact(readyName)
	assert.Error(t, statErr, "SessionStart ready must be consumed before direct prompt completion wait")
}

func TestRecreatePane_StartupArtifactConsumeFailureDoesNotCommit(t *testing.T) {
	term := newRecoveryLaunchTerminal()
	provider := ProviderConfig{
		Name: "claude", Binary: "claude", StartupTimeout: time.Second,
	}
	cfg, session := newRecoveryHookConfig(t, term, "recovery-ready-consume", provider)
	readyName := RoundSignalName(provider.Name, 2, "ready")
	term.onLaunch = func(terminal.PaneID) {
		readyDir := filepath.Join(session.Dir(), readyName)
		require.NoError(t, os.Mkdir(readyDir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(readyDir, "keep"), nil, 0o600))
	}
	old := paneInfo{paneID: "old-pane", provider: provider}

	got, err := recreatePane(context.Background(), cfg, old, 2)

	require.Error(t, err)
	assert.Equal(t, old, got)
	assert.Contains(t, term.closeCalls, "pane-1")
	assert.NotContains(t, term.closeCalls, "old-pane")
}

func TestSurfaceManager_WarmReplacementRequiresFreshStartupArtifact(t *testing.T) {
	term := newRecoveryLaunchTerminal()
	term.name = "cmux"
	term.stalePanes = map[terminal.PaneID]bool{"old-pane": true}
	provider := ProviderConfig{
		Name: "claude-code", Binary: "claude", StartupTimeout: time.Second,
	}
	cfg, session := newRecoveryHookConfig(t, term, "recovery-warm-fresh-ready", provider)
	manager := cfg.SurfaceMgr
	manager.warmPool = &WarmPool{
		term: term, pool: []warmPane{{paneID: "warm-pane"}},
	}
	readyName := RoundSignalName(provider.Name, 2, "ready")
	require.NoError(t, session.writeArtifact(readyName, nil, 0o600))
	staleWasReset := false
	term.onLaunch = func(paneID terminal.PaneID) {
		assert.Equal(t, terminal.PaneID("warm-pane"), paneID)
		_, staleErr := session.statArtifact(readyName)
		staleWasReset = os.IsNotExist(staleErr)
		require.NoError(t, session.writeArtifact(readyName, nil, 0o600))
	}
	old := paneInfo{paneID: "old-pane", provider: provider}

	got, recovered, err := manager.ValidateAndRecover(context.Background(), cfg, old, 2)

	require.NoError(t, err)
	assert.True(t, recovered)
	assert.True(t, staleWasReset)
	assert.Equal(t, terminal.PaneID("warm-pane"), got.paneID)
	assert.Contains(t, term.closeCalls, "old-pane")
	_, statErr := session.statArtifact(readyName)
	assert.Error(t, statErr)
}

func TestRecreatePane_HookModeWithoutActiveSessionFailsClosed(t *testing.T) {
	term := newRecoveryLaunchTerminal()
	old := paneInfo{
		paneID: "old-pane", provider: ProviderConfig{Name: "claude", Binary: "claude"},
	}

	got, err := recreatePane(context.Background(), OrchestraConfig{
		Terminal: term, HookMode: true, SessionID: "missing-active-session",
	}, old, 2)

	require.ErrorContains(t, err, "active hook session")
	assert.Equal(t, old, got)
	assert.Empty(t, term.createdPanes)
	assert.NotContains(t, term.closeCalls, "old-pane")
}
