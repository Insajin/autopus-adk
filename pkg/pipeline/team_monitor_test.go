package pipeline

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// S1: cmux terminal with 3-person team
func TestTeamMonitorSession_Start_Cmux_ThreeMembers(t *testing.T) {
	t.Parallel()
	term := newTeamMock("cmux")
	session := NewTeamMonitorSession("SPEC-TEAM-001", term, []string{"lead", "builder", "guardian"})

	err := session.Start(context.Background())
	require.NoError(t, err)

	// 3 Vertical splits for 3 teammates.
	assert.Equal(t, 3, term.splitCount)
	assert.Len(t, session.Panes(), 3)
	// Tail -f commands sent to each pane (dashboard pane ID is empty so no echo).
	assert.GreaterOrEqual(t, len(term.sentCommands), 3)
	assert.NotEmpty(t, session.LogPath())

	// All agents initialized as pending.
	state := session.State()
	for _, role := range []string{"lead", "builder", "guardian"} {
		assert.Equal(t, "pending", state.Agents[role])
	}

	_ = session.Close(context.Background())
}

// S2: cmux terminal with 4-person team
func TestTeamMonitorSession_Start_Cmux_FourMembers(t *testing.T) {
	t.Parallel()
	term := newTeamMock("cmux")
	session := NewTeamMonitorSession("SPEC-TEAM-002", term, []string{"lead", "b1", "b2", "guardian"})

	err := session.Start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 4, term.splitCount)
	assert.Len(t, session.Panes(), 4)

	_ = session.Close(context.Background())
}

// S4: plain terminal graceful degradation
func TestTeamMonitorSession_Start_PlainTerminal(t *testing.T) {
	t.Parallel()
	term := newTeamMock("plain")
	session := NewTeamMonitorSession("SPEC-TEAM-003", term, []string{"lead", "builder"})

	err := session.Start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, term.splitCount, "plain terminal should not split")
	assert.Empty(t, session.Panes())

	// UpdateAgent should not panic on plain terminal.
	session.UpdateAgent("lead", "running")
	assert.Equal(t, "running", session.State().Agents["lead"])

	_ = session.Close(context.Background())
}

// S5: teammate status update
func TestTeamMonitorSession_UpdateAgent(t *testing.T) {
	t.Parallel()
	term := newTeamMock("cmux")
	session := NewTeamMonitorSession("SPEC-TEAM-004", term, []string{"lead", "builder"})
	require.NoError(t, session.Start(context.Background()))

	session.UpdateAgent("lead", "running")
	assert.Equal(t, "running", session.State().Agents["lead"])

	session.UpdateAgent("lead", "done")
	assert.Equal(t, "done", session.State().Agents["lead"])

	_ = session.Close(context.Background())
}

// S6: pipeline completion cleanup
func TestTeamMonitorSession_Close_CleansUp(t *testing.T) {
	t.Parallel()
	term := newTeamMock("cmux")
	session := NewTeamMonitorSession("SPEC-TEAM-005", term, []string{"lead", "builder"})
	require.NoError(t, session.Start(context.Background()))

	// Verify panes exist before close.
	require.Len(t, session.Panes(), 2)

	err := session.Close(context.Background())
	require.NoError(t, err)

	assert.Nil(t, session.Panes(), "panes should be nil after close")
	assert.Contains(t, term.closedSessions, "SPEC-TEAM-005")
}

// S7: teammate failure (cmux)
func TestTeamMonitorSession_FailedTeammate_Cmux(t *testing.T) {
	t.Parallel()
	term := newTeamMock("cmux")
	session := NewTeamMonitorSession("SPEC-TEAM-006", term, []string{"lead", "builder"})
	require.NoError(t, session.Start(context.Background()))

	session.FailTeammate(context.Background(), "builder", "compilation error")

	assert.Equal(t, "failed", session.State().Agents["builder"])
	// A failure echo command should have been sent to builder's pane.
	var found bool
	for _, cmd := range term.sentCommands {
		if cmd.paneID == session.Panes()[1].PaneID {
			if contains(cmd.cmd, "[FAILED]") {
				found = true
			}
		}
	}
	assert.True(t, found, "failure message should be sent to builder's pane")

	_ = session.Close(context.Background())
}

// S8: existing MonitorSession non-interference
func TestMonitorSession_Unchanged(t *testing.T) {
	t.Parallel()
	term := newTeamMock("cmux")
	session := NewMonitorSession("SPEC-COMPAT", term)

	err := session.Start(context.Background())
	require.NoError(t, err)
	// MonitorSession creates 2 panes (dashboard + log).
	assert.Equal(t, 2, term.splitCount)

	session.UpdateAgent("executor", "running")
	assert.Equal(t, "running", session.State().Agents["executor"])

	require.NoError(t, session.Close(context.Background()))
}

// S14: PipelineMonitor interface compliance
func TestPipelineMonitor_InterfaceCompliance(t *testing.T) {
	t.Parallel()
	term := newTeamMock("cmux")

	var monitor PipelineMonitor

	// TeamMonitorSession implements PipelineMonitor.
	monitor = NewTeamMonitorSession("SPEC-IF-001", term, []string{"lead"})
	assert.NotNil(t, monitor)

	// MonitorSession implements PipelineMonitor.
	monitor = NewMonitorSession("SPEC-IF-002", term)
	assert.NotNil(t, monitor)
}

// S9: SplitPane failure fallback
func TestTeamMonitorSession_Start_SplitPaneFailure(t *testing.T) {
	t.Parallel()
	term := newTeamMock("cmux")
	term.failSplitAfter = 1 // fail on 2nd split

	session := NewTeamMonitorSession("SPEC-FAIL-001", term, []string{"lead", "builder", "guardian"})
	err := session.Start(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "team layout")
}

func TestTeamMonitorSession_IsMultiplexer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		term   string
		expect bool
	}{
		{"cmux", "cmux", true},
		{"tmux", "tmux", true},
		{"plain", "plain", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := NewTeamMonitorSession("TEST", newTeamMock(tt.term), []string{"lead"})
			assert.Equal(t, tt.expect, session.isMultiplexer())
		})
	}
}

func TestTeamMonitorSession_Start_InvalidSpecID(t *testing.T) {
	t.Parallel()
	term := newTeamMock("cmux")
	session := NewTeamMonitorSession("../evil", term, []string{"lead"})

	err := session.Start(context.Background())
	require.Error(t, err)
}

func TestTeamMonitorSession_Close_NilTerminal(t *testing.T) {
	t.Parallel()
	session := &TeamMonitorSession{specID: "TEST"}
	err := session.Close(context.Background())
	require.NoError(t, err)
}

func TestTeamMonitorSession_Close_TerminalError(t *testing.T) {
	t.Parallel()
	term := newTeamMock("cmux")
	term.failCloseErr = fmt.Errorf("close error")
	session := NewTeamMonitorSession("SPEC-CLOSE-ERR", term, []string{"lead"})

	err := session.Close(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close terminal")
}

func TestTeamMonitorSession_LogPath_Empty(t *testing.T) {
	t.Parallel()
	term := newTeamMock("plain")
	session := NewTeamMonitorSession("SPEC-LOG", term, []string{"lead"})
	require.NoError(t, session.Start(context.Background()))
	// Plain terminal doesn't create panes, so LogPath is empty.
	assert.Empty(t, session.LogPath())
}

func TestTeamMonitorSession_Panes_AfterStart(t *testing.T) {
	t.Parallel()
	term := newTeamMock("cmux")
	session := NewTeamMonitorSession("SPEC-PANES", term, []string{"a", "b"})
	require.NoError(t, session.Start(context.Background()))

	panes := session.Panes()
	assert.Len(t, panes, 2)
	assert.Equal(t, "a", panes[0].Role)
	assert.Equal(t, "b", panes[1].Role)

	_ = session.Close(context.Background())
}

func TestTeamMonitorSession_FailTeammate_NonExistentRole(t *testing.T) {
	t.Parallel()
	term := newTeamMock("cmux")
	session := NewTeamMonitorSession("SPEC-NOEXIST", term, []string{"lead"})
	require.NoError(t, session.Start(context.Background()))

	// Should not panic for unknown role.
	session.FailTeammate(context.Background(), "unknown-role", "error")
	assert.Equal(t, "failed", session.State().Agents["unknown-role"])

	_ = session.Close(context.Background())
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
