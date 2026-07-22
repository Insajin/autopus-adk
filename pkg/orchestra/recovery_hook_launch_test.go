package orchestra

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recoveryLaunchEvent struct {
	kind  string
	value string
	pane  terminal.PaneID
}

type recoveryLaunchTerminal struct {
	mockTerminal
	eventsMu      sync.Mutex
	events        []recoveryLaunchEvent
	commandCalls  int
	failCommandAt int
	stalePanes    map[terminal.PaneID]bool
	screens       map[terminal.PaneID][]string
	screenIndexes map[terminal.PaneID]int
	onLaunch      func(terminal.PaneID)
}

func newRecoveryLaunchTerminal() *recoveryLaunchTerminal {
	return &recoveryLaunchTerminal{mockTerminal: mockTerminal{name: "tmux", readScreenOutput: "❯\n"}}
}

func (m *recoveryLaunchTerminal) SendCommand(_ context.Context, paneID terminal.PaneID, command string) error {
	m.eventsMu.Lock()
	defer m.eventsMu.Unlock()
	m.commandCalls++
	m.events = append(m.events, recoveryLaunchEvent{kind: "command", value: command, pane: paneID})
	if m.commandCalls == m.failCommandAt {
		return errors.New("injected recovery command failure")
	}
	return nil
}

func (m *recoveryLaunchTerminal) SendLongText(_ context.Context, paneID terminal.PaneID, text string) error {
	m.eventsMu.Lock()
	m.events = append(m.events, recoveryLaunchEvent{kind: "long_text", value: text, pane: paneID})
	onLaunch := m.onLaunch
	m.eventsMu.Unlock()
	if onLaunch != nil {
		onLaunch(paneID)
	}
	return nil
}

func (m *recoveryLaunchTerminal) ReadScreen(_ context.Context, paneID terminal.PaneID, _ terminal.ReadScreenOpts) (string, error) {
	m.eventsMu.Lock()
	defer m.eventsMu.Unlock()
	if m.stalePanes[paneID] {
		return "", errors.New("stale pane")
	}
	if sequence := m.screens[paneID]; len(sequence) > 0 {
		if m.screenIndexes == nil {
			m.screenIndexes = make(map[terminal.PaneID]int)
		}
		index := m.screenIndexes[paneID] % len(sequence)
		m.screenIndexes[paneID]++
		return sequence[index], nil
	}
	return m.readScreenOutput, nil
}

func (m *recoveryLaunchTerminal) eventsSnapshot() []recoveryLaunchEvent {
	m.eventsMu.Lock()
	defer m.eventsMu.Unlock()
	return append([]recoveryLaunchEvent(nil), m.events...)
}

func TestLaunchRecoveryProvider_ExportsForCompletionOrStartupCapability(t *testing.T) {
	t.Parallel()
	on, off := true, false
	tests := []struct {
		name       string
		provider   ProviderConfig
		wantExport bool
	}{
		{name: "completion default", provider: ProviderConfig{Name: "gemini", Binary: "gemini"}, wantExport: true},
		{name: "startup default", provider: ProviderConfig{Name: "claude", Binary: "claude", HasHook: &off}, wantExport: true},
		{name: "startup override", provider: ProviderConfig{Name: "custom", Binary: "custom", HasHook: &off, HasStartupHook: &on}, wantExport: true},
		{name: "hookless", provider: ProviderConfig{Name: "custom", Binary: "custom", HasHook: &off, HasStartupHook: &off}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			term := newRecoveryLaunchTerminal()
			sessionID := "recovery-capability-" + sanitizeProviderName(tt.name)
			cfg, _ := newRecoveryHookConfig(t, term, sessionID, tt.provider)

			err := launchRecoveryProvider(context.Background(), cfg, "replacement", tt.provider, 4)

			require.NoError(t, err)
			events := term.eventsSnapshot()
			exports := make([]string, 0, 2)
			launchIndex := -1
			for i, event := range events {
				if event.kind == "command" && strings.Contains(event.value, "AUTOPUS_") {
					exports = append(exports, event.value)
				}
				if event.kind == "long_text" {
					launchIndex = i
				}
			}
			require.NotEqual(t, -1, launchIndex)
			if !tt.wantExport {
				assert.Empty(t, exports)
				return
			}
			require.Len(t, exports, 2)
			assert.Contains(t, exports[0], "AUTOPUS_SESSION_ID="+sessionID)
			assert.Equal(t, "export AUTOPUS_ROUND=4", exports[1])
			assert.Equal(t, "\n", events[1].value)
			assert.Equal(t, "\n", events[3].value)
			assert.Greater(t, launchIndex, 3)
		})
	}
}

func TestLaunchRecoveryProvider_HookExportFailureSkipsLaunch(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name          string
		failCommandAt int
		wantError     string
	}{
		{name: "session send", failCommandAt: 1, wantError: "export hook session failed"},
		{name: "session enter", failCommandAt: 2, wantError: "commit hook session export failed"},
		{name: "round send", failCommandAt: 3, wantError: "export hook round failed"},
		{name: "round enter", failCommandAt: 4, wantError: "commit hook round export failed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			term := newRecoveryLaunchTerminal()
			term.failCommandAt = tt.failCommandAt
			provider := ProviderConfig{Name: "claude", Binary: "claude"}
			cfg, _ := newRecoveryHookConfig(
				t, term, "recovery-failure-"+sanitizeProviderName(tt.name), provider,
			)

			err := launchRecoveryProvider(context.Background(), cfg, "replacement", provider, 3)

			require.ErrorContains(t, err, tt.wantError)
			for _, event := range term.eventsSnapshot() {
				assert.NotEqual(t, "long_text", event.kind, "provider must not launch after hook export failure")
			}
		})
	}
}

func TestRecreatePane_HookExportFailureCleansReplacement(t *testing.T) {
	term := newRecoveryLaunchTerminal()
	term.failCommandAt = 1
	old := paneInfo{paneID: "old-pane", provider: ProviderConfig{Name: "claude", Binary: "claude"}}
	cfg, _ := newRecoveryHookConfig(t, term, "recovery-cold-cleanup", old.provider)

	got, err := recreatePane(context.Background(), cfg, old, 2)

	require.ErrorContains(t, err, "export hook session failed")
	assert.Equal(t, old, got)
	assert.Contains(t, term.closeCalls, "pane-1")
	assert.NotContains(t, term.closeCalls, "old-pane")
	assert.Empty(t, term.createdPanes[1:], "hook export failure must not launch a second replacement")
}

func TestSurfaceManager_WarmHookExportFailureFallsBackToColdRecovery(t *testing.T) {
	term := newRecoveryLaunchTerminal()
	term.failCommandAt = 1
	term.stalePanes = map[terminal.PaneID]bool{"old-pane": true}
	term.name = "cmux"
	sm := NewSurfaceManager(term)
	sm.warmPool = &WarmPool{term: term, pool: []warmPane{{paneID: "warm-pane"}}}
	old := paneInfo{paneID: "old-pane", provider: ProviderConfig{Name: "claude", Binary: "claude"}}
	cfg, session := newRecoveryHookConfig(t, term, "recovery-warm-cleanup", old.provider)
	sm.setHookSession(session)
	cfg.SurfaceMgr = sm
	term.onLaunch = func(terminal.PaneID) {
		require.NoError(t, session.writeArtifact(
			RoundSignalName(old.provider.Name, 2, "ready"), nil, 0o600,
		))
	}

	got, recovered, err := sm.ValidateAndRecover(context.Background(), cfg, old, 2)

	require.NoError(t, err)
	assert.True(t, recovered)
	assert.Equal(t, terminal.PaneID("pane-1"), got.paneID)
	assert.Equal(t, 2, got.directPromptRound)
	assert.Contains(t, term.closeCalls, "warm-pane")
	assert.Contains(t, term.closeCalls, "old-pane", "old pane retires only after cold recovery succeeds")
	assert.Equal(t, []terminal.PaneID{"pane-1"}, term.createdPanes)
	for _, event := range term.eventsSnapshot() {
		assert.False(t, event.kind == "long_text" && event.pane == "warm-pane",
			"warm provider must not launch after its hook export fails")
	}
}

func TestRecreatePane_RequiresStableProviderReadyBeforeRetiringOld(t *testing.T) {
	term := newRecoveryLaunchTerminal()
	term.screens = map[terminal.PaneID][]string{"pane-1": {"❯\n", "$ \n"}}
	old := paneInfo{
		paneID:   "old-pane",
		provider: ProviderConfig{Name: "claude", Binary: "claude", StartupTimeout: 450 * time.Millisecond},
	}

	got, err := recreatePane(context.Background(), OrchestraConfig{Terminal: term}, old, 2)

	require.Error(t, err)
	assert.Equal(t, old, got)
	assert.Contains(t, term.closeCalls, "pane-1")
	assert.NotContains(t, term.closeCalls, "old-pane")
}
