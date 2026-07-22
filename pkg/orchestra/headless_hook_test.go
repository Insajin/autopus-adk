package orchestra

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEdge1_SplitPaneFail_DegradeToSubprocess verifies Edge Case 1: when
// splitProviderPanes (SplitPane) fails, RunInteractivePaneOrchestra degrades to
// the subprocess fallback (RunPaneOrchestra) without hanging.
func TestEdge1_SplitPaneFail_DegradeToSubprocess(t *testing.T) {
	t.Parallel()

	// Mock terminal whose SplitPane always fails.
	mock := &mockTerminal{
		name:         "cmux",
		splitPaneErr: fmt.Errorf("no space for new pane"),
	}

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "echo", PaneArgs: []string{"hello"}},
		},
		Strategy:       StrategyFastest,
		Prompt:         "test prompt",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		HookMode:       false,
	}

	// 10s wall-clock limit — the oracle is "no infinite hang", not "must succeed".
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// RunInteractivePaneOrchestra must not hang (degrade path calls RunPaneOrchestra).
	// The subprocess backend may error because 'echo' produces no sentinel output,
	// so we allow both (result, nil) and (nil, err) — the key assertion is that
	// SplitPane was attempted and the call returned within the context deadline.
	done := make(chan struct{})
	var callResult *OrchestraResult
	var callErr error
	go func() {
		callResult, callErr = RunInteractivePaneOrchestra(ctx, cfg)
		close(done)
	}()
	select {
	case <-done:
		// Call completed — assert that SplitPane was attempted (failure branch hit).
		assert.GreaterOrEqual(t, len(mock.splitPaneCalls), 1, "Edge1: SplitPane must have been attempted")
		// Either a result or an error is acceptable — no hang is the invariant.
		assert.True(t, callResult != nil || callErr != nil,
			"Edge1: call must have completed with result or error")
	case <-ctx.Done():
		t.Fatal("Edge1: RunInteractivePaneOrchestra hung — did not return within 10s")
	}
	_ = callResult
}

// TestSendSessionEnvToPane_ValidID verifies that a valid session ID produces
// an "export AUTOPUS_SESSION_ID=<id>" command on the terminal pane.
func TestSendSessionEnvToPane_ValidID(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	ctx := context.Background()

	err := SendSessionEnvToPane(ctx, mock, "pane-1", "orch-abc123-def")
	require.NoError(t, err, "valid session ID must not return error")

	// Verify the export command was captured by the mock.
	mock.mu.Lock()
	defer mock.mu.Unlock()
	found := false
	for _, call := range mock.sendCommandCalls {
		if strings.Contains(call.Cmd, "export AUTOPUS_SESSION_ID=orch-abc123-def") {
			found = true
			break
		}
	}
	assert.True(t, found, "SendSessionEnvToPane must send 'export AUTOPUS_SESSION_ID=orch-abc123-def'")
}

// TestSendSessionEnvToPane_InvalidID_Rejected verifies that session IDs with
// shell-unsafe characters are rejected (injection prevention).
func TestSendSessionEnvToPane_InvalidID_Rejected(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	ctx := context.Background()

	cases := []string{
		"Orch-ABC",
		"sid with space",
		"sid;rm -rf /",
		"sid$(whoami)",
		"sid|pipe",
		"../traversal",
	}
	for _, id := range cases {
		err := SendSessionEnvToPane(ctx, mock, "pane-1", id)
		assert.Error(t, err, "invalid session ID %q must be rejected", id)
		assert.Contains(t, err.Error(), "invalid session ID", "error must mention invalid session ID")
	}

	// No SendCommand must have been issued for any invalid ID.
	mock.mu.Lock()
	defer mock.mu.Unlock()
	assert.Empty(t, mock.sendCommandCalls, "no SendCommand must be issued for invalid session IDs")
}

// TestLaunchInteractiveSessions_SessionEnvExported verifies that when HookMode
// is enabled, SendSessionEnvToPane is called for each pane BEFORE the launch
// command, so cmux surfaces inherit AUTOPUS_SESSION_ID.
func TestLaunchInteractiveSessions_SessionEnvExported(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.readScreenOutput = "❯"
	sessionID := "orch-test-sessionenv"

	provider := ProviderConfig{Name: "claude", Binary: "echo", InteractiveInput: "stdin"}
	panes := []paneInfo{
		{paneID: "pane-1", provider: provider},
	}

	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{provider},
		Strategy:       StrategyFastest,
		Prompt:         "test",
		TimeoutSeconds: 5,
		Terminal:       mock,
		HookMode:       true,
		SessionID:      sessionID,
	}

	ctx := context.Background()
	_ = launchInteractiveSessions(ctx, cfg, panes)

	// Verify that "export AUTOPUS_SESSION_ID=<id>" appeared in SendCommand calls.
	mock.mu.Lock()
	defer mock.mu.Unlock()
	envExported := false
	for _, call := range mock.sendCommandCalls {
		if strings.Contains(call.Cmd, "export AUTOPUS_SESSION_ID="+sessionID) {
			envExported = true
			break
		}
	}
	assert.True(t, envExported,
		"launchInteractiveSessions must send 'export AUTOPUS_SESSION_ID=%s' to each pane when HookMode=true", sessionID)
}

// TestLaunchInteractiveSessions_NoEnvWhenHookModeOff verifies that when
// HookMode is false, AUTOPUS_SESSION_ID is NOT exported to panes.
func TestLaunchInteractiveSessions_NoEnvWhenHookModeOff(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.readScreenOutput = "❯"

	provider := ProviderConfig{Name: "claude", Binary: "echo", InteractiveInput: "stdin"}
	panes := []paneInfo{
		{paneID: "pane-1", provider: provider},
	}

	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{provider},
		Strategy:       StrategyFastest,
		Prompt:         "test",
		TimeoutSeconds: 5,
		Terminal:       mock,
		HookMode:       false,
		SessionID:      "orch-should-not-appear",
	}

	ctx := context.Background()
	_ = launchInteractiveSessions(ctx, cfg, panes)

	mock.mu.Lock()
	defer mock.mu.Unlock()
	for _, call := range mock.sendCommandCalls {
		assert.NotContains(t, call.Cmd, "AUTOPUS_SESSION_ID",
			"AUTOPUS_SESSION_ID must NOT be exported when HookMode=false")
	}
}
