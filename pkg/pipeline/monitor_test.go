package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// mockTerminal is a test double for the terminal.Terminal interface.
type mockTerminal struct {
	name       string
	splitCount int
	closed     bool
}

func (m *mockTerminal) Name() string { return m.name }
func (m *mockTerminal) CreateWorkspace(_ context.Context, _ string) error {
	return nil
}
func (m *mockTerminal) SplitPane(_ context.Context, _ terminal.Direction) (terminal.PaneID, error) {
	m.splitCount++
	return terminal.PaneID("pane-" + string(rune('0'+m.splitCount))), nil
}
func (m *mockTerminal) SendCommand(_ context.Context, _ terminal.PaneID, _ string) error {
	return nil
}
func (m *mockTerminal) Notify(_ context.Context, _ string) error { return nil }
func (m *mockTerminal) Close(_ context.Context, _ string) error {
	m.closed = true
	return nil
}
func (m *mockTerminal) ReadScreen(_ context.Context, _ terminal.PaneID, _ terminal.ReadScreenOpts) (string, error) {
	return "", nil
}
func (m *mockTerminal) PipePaneStart(_ context.Context, _ terminal.PaneID, _ string) error {
	return nil
}
func (m *mockTerminal) PipePaneStop(_ context.Context, _ terminal.PaneID) error { return nil }

// TestMonitorSession_Start_WithCmux verifies that Start creates 2 panes
// when a cmux terminal is used.
func TestMonitorSession_Start_WithCmux(t *testing.T) {
	t.Parallel()

	term := &mockTerminal{name: "cmux"}
	session := NewMonitorSession("SPEC-ORCH-002", term)

	err := session.Start(context.Background())
	require.NoError(t, err)

	// With cmux, Start should create exactly 2 panes.
	assert.Equal(t, 2, term.splitCount,
		"Start with cmux should create 2 panes (dashboard + log tail)")
}

// TestMonitorSession_Start_WithoutCmux verifies that Start skips pane
// creation gracefully when a plain terminal is used.
func TestMonitorSession_Start_WithoutCmux(t *testing.T) {
	t.Parallel()

	term := &mockTerminal{name: "plain"}
	session := NewMonitorSession("SPEC-ORCH-002", term)

	err := session.Start(context.Background())
	require.NoError(t, err)

	// With plain terminal, no panes should be created.
	assert.Equal(t, 0, term.splitCount,
		"Start with plain terminal should not create panes")
}

// TestMonitorSession_Close verifies that Close cleans up panes and log files.
func TestMonitorSession_Close(t *testing.T) {
	t.Parallel()

	term := &mockTerminal{name: "cmux"}
	session := NewMonitorSession("SPEC-ORCH-002", term)

	err := session.Close(context.Background())
	require.NoError(t, err)

	assert.True(t, term.closed,
		"Close should call terminal Close to clean up")
}

// TestMonitorSession_LogPath verifies the correct log file path is returned.
func TestMonitorSession_LogPath(t *testing.T) {
	t.Parallel()

	term := &mockTerminal{name: "cmux"}
	session := NewMonitorSession("SPEC-ORCH-002", term)

	logPath := session.LogPath()
	assert.NotEmpty(t, logPath,
		"LogPath should return a non-empty path")
	assert.Contains(t, logPath, "SPEC-ORCH-002",
		"LogPath should contain the spec ID")
}

// TestMonitorSession_State verifies the state model tracks phase and agents
// independently of panes.
func TestMonitorSession_State(t *testing.T) {
	t.Parallel()

	term := &mockTerminal{name: "cmux"}
	session := NewMonitorSession("SPEC-ORCH-002", term)

	state := session.State()

	// State should have initialized Agents map.
	assert.NotNil(t, state.Agents,
		"State().Agents should be initialized (non-nil)")

	// Since no phase has been set, it should be empty.
	assert.Empty(t, state.Phase, "initial Phase should be empty")
}

// errorTerminal simulates SplitPane failures for testing error paths.
type errorTerminal struct {
	name      string
	failAfter int // fail after N successful splits
	splits    int
	closed    bool
}

func (e *errorTerminal) Name() string { return e.name }
func (e *errorTerminal) CreateWorkspace(_ context.Context, _ string) error {
	return nil
}
func (e *errorTerminal) SplitPane(_ context.Context, _ terminal.Direction) (terminal.PaneID, error) {
	e.splits++
	if e.splits > e.failAfter {
		return "", fmt.Errorf("split pane failure")
	}
	return terminal.PaneID(fmt.Sprintf("pane-%d", e.splits)), nil
}
func (e *errorTerminal) SendCommand(_ context.Context, _ terminal.PaneID, _ string) error {
	return nil
}
func (e *errorTerminal) Notify(_ context.Context, _ string) error { return nil }
func (e *errorTerminal) Close(_ context.Context, _ string) error {
	e.closed = true
	return nil
}
func (e *errorTerminal) ReadScreen(_ context.Context, _ terminal.PaneID, _ terminal.ReadScreenOpts) (string, error) {
	return "", nil
}
func (e *errorTerminal) PipePaneStart(_ context.Context, _ terminal.PaneID, _ string) error {
	return nil
}
func (e *errorTerminal) PipePaneStop(_ context.Context, _ terminal.PaneID) error { return nil }

// TestMonitorSession_Start_FirstSplitError verifies Start returns error
// when the first SplitPane call fails.
func TestMonitorSession_Start_FirstSplitError(t *testing.T) {
	t.Parallel()

	term := &errorTerminal{name: "cmux", failAfter: 0}
	session := NewMonitorSession("SPEC-ERR-001", term)

	err := session.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "log pane")
}

// TestMonitorSession_Start_SecondSplitError verifies Start returns error
// when the second SplitPane call fails.
func TestMonitorSession_Start_SecondSplitError(t *testing.T) {
	t.Parallel()

	term := &errorTerminal{name: "cmux", failAfter: 1}
	session := NewMonitorSession("SPEC-ERR-002", term)

	err := session.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dashboard pane")
}

// TestMonitorSession_Close_WithNilTerm verifies Close handles nil terminal gracefully.
func TestMonitorSession_Close_WithNilTerm(t *testing.T) {
	t.Parallel()

	session := &MonitorSession{
		specID:  "SPEC-NIL",
		term:    nil,
		logPath: filepath.Join(t.TempDir(), "nonexistent.log"),
	}

	err := session.Close(context.Background())
	assert.NoError(t, err, "Close with nil terminal should not error")
}

// failCloseTerminal is a terminal that returns an error on Close.
type failCloseTerminal struct {
	mockTerminal
}

func (f *failCloseTerminal) Close(_ context.Context, _ string) error {
	return fmt.Errorf("terminal close failure")
}

// TestMonitorSession_Close_TerminalError verifies Close propagates terminal close errors.
func TestMonitorSession_Close_TerminalError(t *testing.T) {
	t.Parallel()

	term := &failCloseTerminal{mockTerminal: mockTerminal{name: "cmux"}}
	session := NewMonitorSession("SPEC-ERR-003", term)

	err := session.Close(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close terminal")
}

// TestMonitorSession_IsCmux verifies isCmux detection logic.
func TestMonitorSession_IsCmux(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		term   terminal.Terminal
		expect bool
	}{
		{"cmux", &mockTerminal{name: "cmux"}, true},
		{"tmux", &mockTerminal{name: "tmux"}, false},
		{"plain", &mockTerminal{name: "plain"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := NewMonitorSession("TEST", tt.term)
			assert.Equal(t, tt.expect, session.isCmux())
		})
	}
}
