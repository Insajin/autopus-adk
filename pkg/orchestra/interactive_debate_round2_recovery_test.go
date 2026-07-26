package orchestra

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type round2ReleaseRecoveryTerminal struct {
	*recoveryLaunchTerminal
	orderMu sync.Mutex
	order   []string
}

func newRound2ReleaseRecoveryTerminal() *round2ReleaseRecoveryTerminal {
	return &round2ReleaseRecoveryTerminal{recoveryLaunchTerminal: newRecoveryLaunchTerminal()}
}

func (m *round2ReleaseRecoveryTerminal) SplitPane(
	ctx context.Context,
	dir terminal.Direction,
) (terminal.PaneID, error) {
	m.recordOrder("split")
	return m.recoveryLaunchTerminal.SplitPane(ctx, dir)
}

func (m *round2ReleaseRecoveryTerminal) Close(ctx context.Context, ref string) error {
	m.recordOrder("close:" + ref)
	return m.recoveryLaunchTerminal.Close(ctx, ref)
}

func (m *round2ReleaseRecoveryTerminal) SendLongText(
	ctx context.Context,
	paneID terminal.PaneID,
	text string,
) error {
	m.recordOrder("long_text:" + string(paneID))
	return m.recoveryLaunchTerminal.SendLongText(ctx, paneID, text)
}

func (m *round2ReleaseRecoveryTerminal) recordOrder(event string) {
	m.orderMu.Lock()
	defer m.orderMu.Unlock()
	m.order = append(m.order, event)
}

func (m *round2ReleaseRecoveryTerminal) orderIndex(event string) int {
	m.orderMu.Lock()
	defer m.orderMu.Unlock()
	for i, got := range m.order {
		if got == event {
			return i
		}
	}
	return -1
}

func (m *round2ReleaseRecoveryTerminal) orderCount(event string) int {
	m.orderMu.Lock()
	defer m.orderMu.Unlock()
	count := 0
	for _, got := range m.order {
		if got == event {
			count++
		}
	}
	return count
}

func TestExecuteRound_FileIPCReleaseFailureRetiresOldPaneBeforeRecovery(t *testing.T) {
	term := newRound2ReleaseRecoveryTerminal()
	cfg, session, provider := newRound2ReleaseRecoveryFixture(t, term)
	longTextCount := 0
	term.onLaunch = func(paneID terminal.PaneID) {
		assert.Equal(t, terminal.PaneID("pane-1"), paneID)
		longTextCount++
		switch longTextCount {
		case 1:
			require.NoError(t, session.writeArtifact(
				RoundSignalName(provider.Name, 2, "ready"), nil, 0o600,
			))
		case 2:
			require.NoError(t, session.writeJSONArtifact(
				RoundSignalName(provider.Name, 2, "result.json"),
				HookResult{Output: "replacement response"},
			))
			require.NoError(t, session.writeArtifact(
				RoundSignalName(provider.Name, 2, "done"), nil, 0o600,
			))
		}
	}

	responses := executeRound(
		context.Background(), cfg,
		[]paneInfo{{provider: provider, paneID: "old-pane"}},
		session, 2,
		[]ProviderResponse{{Provider: "codex", Output: "round one"}},
	)

	require.Len(t, responses, 1)
	assert.Equal(t, "replacement response", responses[0].Output)
	closeIndex := term.orderIndex("close:old-pane")
	splitIndex := term.orderIndex("split")
	require.NotEqual(t, -1, closeIndex)
	require.NotEqual(t, -1, splitIndex)
	assert.Less(t, closeIndex, splitIndex,
		"old hook owner must close before replacement can reset shared artifacts")
	assert.Equal(t, 1, term.orderIndex("long_text:pane-1")-splitIndex)
	assert.Equal(t, 2, longTextCount, "replacement must receive one launch and one prompt")
	assert.Equal(t, -1, term.orderIndex("long_text:old-pane"))
}

func TestExecuteRound_FileIPCReleaseFailureCloseFailureStaysFailClosed(t *testing.T) {
	term := newRound2ReleaseRecoveryTerminal()
	term.closeErr = errors.New("injected close failure")
	cfg, session, provider := newRound2ReleaseRecoveryFixture(t, term)

	responses := executeRound(
		context.Background(), cfg,
		[]paneInfo{{provider: provider, paneID: "old-pane"}},
		session, 2,
		[]ProviderResponse{{Provider: "codex", Output: "round one"}},
	)

	require.Len(t, responses, 1)
	assert.Equal(t, skippedHookCollectionError, responses[0].Error)
	assert.Equal(t, closePaneSurfaceAttempts, term.orderCount("close:old-pane"))
	assert.Equal(t, -1, term.orderIndex("split"))
	assert.Equal(t, -1, term.orderIndex("long_text:old-pane"))
	assert.Equal(t, -1, term.orderIndex("long_text:pane-1"))
}

func TestExecuteRound_FileIPCReleaseFailureReplacementFailureStaysFailClosed(t *testing.T) {
	term := newRound2ReleaseRecoveryTerminal()
	term.splitPaneErr = errors.New("injected replacement failure")
	cfg, session, provider := newRound2ReleaseRecoveryFixture(t, term)

	responses := executeRound(
		context.Background(), cfg,
		[]paneInfo{{provider: provider, paneID: "old-pane"}},
		session, 2,
		[]ProviderResponse{{Provider: "codex", Output: "round one"}},
	)

	require.Len(t, responses, 1)
	assert.Equal(t, skippedHookCollectionError, responses[0].Error)
	closeIndex := term.orderIndex("close:old-pane")
	splitIndex := term.orderIndex("split")
	require.NotEqual(t, -1, closeIndex)
	require.NotEqual(t, -1, splitIndex)
	assert.Less(t, closeIndex, splitIndex)
	assert.Equal(t, -1, term.orderIndex("long_text:old-pane"))
	assert.Equal(t, -1, term.orderIndex("long_text:pane-1"))
}

func newRound2ReleaseRecoveryFixture(
	t *testing.T,
	term terminal.Terminal,
) (OrchestraConfig, *HookSession, ProviderConfig) {
	t.Helper()
	provider := ProviderConfig{
		Name: "claude", Binary: "echo", InteractiveInput: "stdin",
		StartupTimeout: time.Second,
	}
	cfg, session := newRecoveryHookConfig(
		t, term, "ipc-release-recovery-"+NewSessionID(), provider,
	)
	cfg.Providers = []ProviderConfig{provider}
	cfg.Strategy = StrategyDebate
	cfg.Prompt = "recover current round"
	cfg.TimeoutSeconds = 5
	cfg.Interactive = true
	cfg.InitialDelay = time.Millisecond
	cfg.WorkingDir = t.TempDir()

	require.NoError(t, session.writeArtifact(
		RoundSignalName(provider.Name, 2, "ready"), nil, 0o600,
	))
	require.NoError(t, os.Mkdir(
		filepath.Join(session.Dir(), RoundSignalName(provider.Name, 2, "input.json")+".tmp"), 0o700,
	))
	require.NoError(t, os.Mkdir(
		filepath.Join(session.Dir(), RoundSignalName(provider.Name, 2, "abort")), 0o700,
	))
	return cfg, session, provider
}
