package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkerModel_Defaults(t *testing.T) {
	t.Parallel()

	m := NewWorkerModel()

	assert.Equal(t, ConnDisconnected, m.connStatus)
	assert.Equal(t, 80, m.width)
	assert.Equal(t, 24, m.height)
	assert.Nil(t, m.currentTask)
	assert.Nil(t, m.approval)
	assert.Empty(t, m.taskQueue)
	assert.Empty(t, m.providers)
	assert.False(t, m.showDetail)
	assert.Equal(t, float64(0), m.costTracker.DailyCost)
}

func TestInit_ReturnsNil(t *testing.T) {
	t.Parallel()

	m := NewWorkerModel()
	assert.Nil(t, m.Init())
}

func TestUpdate_WindowSizeMsg(t *testing.T) {
	t.Parallel()

	m := NewWorkerModel()
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	assert.Nil(t, cmd)
	wm := updated.(WorkerModel)
	assert.Equal(t, 120, wm.width)
	assert.Equal(t, 40, wm.height)
}

func TestUpdate_ConnStatusMsg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status ConnStatus
	}{
		{"connected", ConnConnected},
		{"disconnected", ConnDisconnected},
		{"reconnecting", ConnReconnecting},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := NewWorkerModel()
			updated, _ := m.Update(ConnStatusMsg(tt.status))
			wm := updated.(WorkerModel)
			assert.Equal(t, tt.status, wm.connStatus)
		})
	}
}

func TestUpdate_TaskReceivedMsg(t *testing.T) {
	t.Parallel()

	m := NewWorkerModel()
	msg := TaskReceivedMsg{ID: "task-1", Description: "test task", Status: "pending"}
	updated, _ := m.Update(msg)
	wm := updated.(WorkerModel)

	require.Len(t, wm.taskQueue, 1)
	assert.Equal(t, "task-1", wm.taskQueue[0].ID)
	assert.Equal(t, "test task", wm.taskQueue[0].Description)
}

func TestUpdate_TaskProgressMsg(t *testing.T) {
	t.Parallel()

	m := NewWorkerModel()
	m.currentTask = &CurrentTask{ID: "task-1", Phase: "plan", Progress: 0.0}

	updated, _ := m.Update(TaskProgressMsg{Phase: "execute", Progress: 0.5})
	wm := updated.(WorkerModel)

	require.NotNil(t, wm.currentTask)
	assert.Equal(t, "execute", wm.currentTask.Phase)
	assert.Equal(t, 0.5, wm.currentTask.Progress)
}

func TestUpdate_TaskProgressMsg_NoCurrentTask(t *testing.T) {
	t.Parallel()

	m := NewWorkerModel()
	updated, _ := m.Update(TaskProgressMsg{Phase: "execute", Progress: 0.5})
	wm := updated.(WorkerModel)
	assert.Nil(t, wm.currentTask)
}

func TestUpdate_TaskCompleteMsg(t *testing.T) {
	t.Parallel()

	m := NewWorkerModel()
	m.currentTask = &CurrentTask{ID: "task-1"}
	m.costTracker.DailyCost = 1.00

	updated, _ := m.Update(TaskCompleteMsg{CostUSD: 0.25})
	wm := updated.(WorkerModel)

	assert.Nil(t, wm.currentTask)
	assert.InDelta(t, 1.25, wm.costTracker.DailyCost, 0.001)
}

func TestUpdate_ApprovalRequestMsg(t *testing.T) {
	t.Parallel()

	m := NewWorkerModel()
	msg := ApprovalRequestMsg{TaskID: "task-99", Action: "deploy", RiskLevel: "high", Context: "prod"}
	updated, _ := m.Update(msg)
	wm := updated.(WorkerModel)

	require.NotNil(t, wm.approval)
	assert.Equal(t, "deploy", wm.approval.Action)
	assert.Equal(t, "high", wm.approval.RiskLevel)
}

func TestHandleKey_Quit(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"q", "ctrl+c"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			m := NewWorkerModel()
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
			if key == "ctrl+c" {
				_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			}
			// tea.Quit returns a non-nil Cmd
			if key == "q" {
				assert.NotNil(t, cmd)
			}
		})
	}
}

func TestHandleKey_ToggleDetail(t *testing.T) {
	t.Parallel()

	m := NewWorkerModel()
	assert.False(t, m.showDetail)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	wm := updated.(WorkerModel)
	assert.True(t, wm.showDetail)

	updated, _ = wm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	wm = updated.(WorkerModel)
	assert.False(t, wm.showDetail)
}

func TestHandleKey_CancelTask(t *testing.T) {
	t.Parallel()

	m := NewWorkerModel()
	m.currentTask = &CurrentTask{ID: "task-1"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	wm := updated.(WorkerModel)
	assert.Nil(t, wm.currentTask)
}

func TestHandleKey_ApprovalMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		key            rune
		clearsAppr     bool
		expectDecision string
		expectViewDiff bool
	}{
		{"approve", 'a', true, "approve", false},
		{"deny", 'd', true, "deny", false},
		{"view", 'v', false, "", true},
		{"skip", 's', true, "skip", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := NewWorkerModel()
			m.approval = &ApprovalRequest{TaskID: "task-42", Action: "test", RiskLevel: "low"}

			var gotTaskID, gotDecision string
			var viewDiffCalled bool
			m.OnApprovalDecision = func(taskID, decision string) {
				gotTaskID = taskID
				gotDecision = decision
			}
			m.OnViewDiff = func(taskID string) {
				viewDiffCalled = true
				gotTaskID = taskID
			}

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}})
			wm := updated.(WorkerModel)

			if tt.clearsAppr {
				assert.Nil(t, wm.approval)
			} else {
				assert.NotNil(t, wm.approval)
			}

			if tt.expectDecision != "" {
				assert.Equal(t, "task-42", gotTaskID)
				assert.Equal(t, tt.expectDecision, gotDecision)
			}
			if tt.expectViewDiff {
				assert.True(t, viewDiffCalled)
				assert.Equal(t, "task-42", gotTaskID)
			}
		})
	}
}

func TestView_ContainsExpectedSections(t *testing.T) {
	t.Parallel()

	m := NewWorkerModel()
	view := m.View()

	assert.Contains(t, view, "[q]uit")
	assert.Contains(t, view, "autopus worker")
}
