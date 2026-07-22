package orchestra

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type launchOrderEvent struct {
	kind  string
	value string
}

type launchOrderTerminal struct {
	*seqScreenMock
	events []launchOrderEvent
}

func (m *launchOrderTerminal) SendCommand(_ context.Context, _ terminal.PaneID, cmd string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, launchOrderEvent{kind: "command", value: cmd})
	return nil
}

func (m *launchOrderTerminal) SendLongText(_ context.Context, _ terminal.PaneID, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, launchOrderEvent{kind: "long_text", value: text})
	return nil
}

func (m *launchOrderTerminal) eventsSnapshot() []launchOrderEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]launchOrderEvent(nil), m.events...)
}

func TestLaunchInteractiveSessions_DebateExportsRoundBeforeLaunch(t *testing.T) {
	t.Parallel()

	term := &launchOrderTerminal{seqScreenMock: &seqScreenMock{name: "cmux"}}
	provider := ProviderConfig{Name: "claude", Binary: "claude"}
	panes := []paneInfo{{provider: provider, paneID: terminal.PaneID("surface:debate-launch")}}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{provider},
		Strategy:  StrategyDebate,
		Terminal:  term,
		HookMode:  true,
		SessionID: "orch-debate-launch-order",
	}

	failed := launchInteractiveSessions(context.Background(), cfg, panes)

	assert.Empty(t, failed)
	events := term.eventsSnapshot()
	sessionIndex, roundIndex, launchIndex := -1, -1, -1
	for i, event := range events {
		switch {
		case event.kind == "command" && event.value == "export AUTOPUS_ROUND=1":
			roundIndex = i
		case event.kind == "command" && strings.Contains(event.value, "export AUTOPUS_SESSION_ID=orch-debate-launch-order"):
			sessionIndex = i
		case event.kind == "long_text":
			launchIndex = i
		}
	}
	require.NotEqual(t, -1, sessionIndex, "session export must be sent")
	require.NotEqual(t, -1, roundIndex, "debate round export must be sent")
	require.NotEqual(t, -1, launchIndex, "provider launch must be sent")
	assert.Less(t, sessionIndex, roundIndex)
	assert.Less(t, roundIndex, launchIndex, "round 1 must be inherited by the provider CLI")
}

func TestLaunchInteractiveSessions_FastestKeepsUnscopedHookRound(t *testing.T) {
	t.Parallel()

	term := &launchOrderTerminal{seqScreenMock: &seqScreenMock{name: "cmux"}}
	provider := ProviderConfig{Name: "claude", Binary: "claude"}
	panes := []paneInfo{{provider: provider, paneID: terminal.PaneID("surface:fastest-launch")}}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{provider},
		Strategy:  StrategyFastest,
		Terminal:  term,
		HookMode:  true,
		SessionID: "orch-fastest-launch-order",
	}

	failed := launchInteractiveSessions(context.Background(), cfg, panes)

	assert.Empty(t, failed)
	events := term.eventsSnapshot()
	sessionExported := false
	for _, event := range events {
		if event.kind != "command" {
			continue
		}
		if strings.Contains(event.value, "export AUTOPUS_SESSION_ID=orch-fastest-launch-order") {
			sessionExported = true
		}
		assert.NotContains(t, event.value, "AUTOPUS_ROUND=",
			"fastest completion still consumes unscoped hook artifacts")
	}
	assert.True(t, sessionExported)
}

func TestLaunchInteractiveSessions_ExportsForCompletionOrStartupCapability(t *testing.T) {
	t.Parallel()
	on, off := true, false
	tests := []struct {
		name     string
		provider ProviderConfig
	}{
		{name: "gemini completion default", provider: ProviderConfig{Name: "gemini", Binary: "gemini"}},
		{name: "claude alias defaults", provider: ProviderConfig{Name: "claude-code", Binary: "claude"}},
		{name: "gemini cli alias discovered", provider: ProviderConfig{Name: "gemini-cli", Binary: "gemini", HasHook: &on}},
		{name: "antigravity alias discovered", provider: ProviderConfig{Name: "antigravity-cli", Binary: "agy", HasHook: &on}},
		{name: "agy alias discovered", provider: ProviderConfig{Name: "agy", Binary: "agy", HasHook: &on}},
		{name: "opencode completion override", provider: ProviderConfig{Name: "opencode", Binary: "opencode", HasHook: &on}},
		{name: "claude startup default", provider: ProviderConfig{Name: "claude", Binary: "claude", HasHook: &off}},
		{name: "custom startup override", provider: ProviderConfig{Name: "custom", Binary: "custom", HasHook: &off, HasStartupHook: &on}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			term := &launchOrderTerminal{seqScreenMock: &seqScreenMock{name: "cmux"}}
			panes := []paneInfo{{provider: tt.provider, paneID: "surface:capability-export"}}
			cfg := OrchestraConfig{
				Providers: []ProviderConfig{tt.provider}, Strategy: StrategyFastest,
				Terminal: term, HookMode: true, SessionID: "orch-capability-export",
			}

			failed := launchInteractiveSessions(context.Background(), cfg, panes)

			assert.Empty(t, failed)
			exported := false
			for _, event := range term.eventsSnapshot() {
				if event.kind == "command" && strings.Contains(event.value, "export AUTOPUS_SESSION_ID=orch-capability-export") {
					exported = true
				}
			}
			assert.True(t, exported, "either completion or startup hooks require the pane session environment")
		})
	}
}

func TestLaunchInteractiveSessions_HookExportFailureSkipsLaunch(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		successBefore int
		wantError     string
	}{
		{name: "session send", successBefore: 0, wantError: "export hook session failed"},
		{name: "session enter", successBefore: 1, wantError: "commit hook session export failed"},
		{name: "round send", successBefore: 2, wantError: "export hook round failed"},
		{name: "round enter", successBefore: 3, wantError: "commit hook round export failed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			term := newCmuxMock()
			term.sendCommandErr = errors.New("injected transport failure")
			term.sendCommandErrAfter = tt.successBefore
			provider := ProviderConfig{Name: "claude", Binary: "claude"}
			panes := []paneInfo{{provider: provider, paneID: "surface:hook-export"}}
			cfg := OrchestraConfig{
				Providers: []ProviderConfig{provider}, Strategy: StrategyDebate,
				Terminal: term, HookMode: true, SessionID: "orch-hook-export-failure",
			}

			failed := launchInteractiveSessions(context.Background(), cfg, panes)

			require.Len(t, failed, 1)
			assert.ErrorContains(t, errors.New(failed[0].Error), tt.wantError)
			assert.True(t, panes[0].skipWait)
			assert.Empty(t, term.sendLongTextCalls, "a provider must not launch without its mandatory hook environment")
		})
	}
}
