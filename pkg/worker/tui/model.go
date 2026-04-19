// Package tui provides a bubbletea-based dashboard for the worker daemon.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ConnStatus represents the connection state to the server.
type ConnStatus int

const (
	ConnConnected ConnStatus = iota
	ConnDisconnected
	ConnReconnecting
)

// ProviderInfo holds status for a registered AI provider.
type ProviderInfo struct {
	Name      string
	Available bool
}

// TaskInfo represents a queued task summary.
type TaskInfo struct {
	ID          string
	Description string
	Status      string
}

// CurrentTask holds the actively executing task state.
type CurrentTask struct {
	ID           string
	Phase        string
	Progress     float64
	SubagentTree string
}

// CostTracker accumulates token cost information.
type CostTracker struct {
	CurrentTaskCost float64
	DailyCost       float64
}

// ConnStatusMsg signals a connection state change.
type ConnStatusMsg ConnStatus

// TaskReceivedMsg signals a new task was received.
type TaskReceivedMsg TaskInfo

// TaskProgressMsg signals progress on the current task.
type TaskProgressMsg struct {
	Phase    string
	Progress float64
}

// TaskCompleteMsg signals the current task finished.
type TaskCompleteMsg struct {
	CostUSD float64
}

// ApprovalRequestMsg signals an approval is needed.
type ApprovalRequestMsg ApprovalRequest

// WorkerModel is the main bubbletea model for the worker dashboard.
type WorkerModel struct {
	connStatus         ConnStatus
	providers          []ProviderInfo
	taskQueue          []TaskInfo
	currentTask        *CurrentTask
	costTracker        CostTracker
	showDetail         bool
	paused             bool
	approval           *ApprovalRequest
	width              int
	height             int
	OnApprovalDecision func(taskID, decision string)
	OnViewDiff         func(taskID string)
	OnPauseToggle      func()
	OnCancelTask       func(taskID string)
}

// NewWorkerModel creates a fresh dashboard model with default state.
func NewWorkerModel() WorkerModel {
	return WorkerModel{
		connStatus: ConnDisconnected,
		width:      80,
		height:     24,
	}
}

// Init returns the initial command (none).
func (m WorkerModel) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and key bindings.
func (m WorkerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		return m.handleKey(msg)
	case ConnStatusMsg:
		m.connStatus = ConnStatus(msg)
	case TaskReceivedMsg:
		m.taskQueue = append(m.taskQueue, TaskInfo(msg))
	case TaskProgressMsg:
		if m.currentTask != nil {
			m.currentTask.Phase = msg.Phase
			m.currentTask.Progress = msg.Progress
		}
	case TaskCompleteMsg:
		m.costTracker.DailyCost += msg.CostUSD
		m.currentTask = nil
	case ApprovalRequestMsg:
		req := ApprovalRequest(msg)
		m.approval = &req
	}
	return m, nil
}

func (m WorkerModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Approval mode takes priority
	if m.approval != nil {
		switch key {
		case "a":
			if m.OnApprovalDecision != nil {
				m.OnApprovalDecision(m.approval.TaskID, "approve")
			}
			m.approval = nil
			return m, nil
		case "d":
			if m.OnApprovalDecision != nil {
				m.OnApprovalDecision(m.approval.TaskID, "deny")
			}
			m.approval = nil
			return m, nil
		case "v":
			if m.OnViewDiff != nil {
				m.OnViewDiff(m.approval.TaskID)
			}
			return m, nil
		case "s":
			if m.OnApprovalDecision != nil {
				m.OnApprovalDecision(m.approval.TaskID, "skip")
			}
			m.approval = nil
			return m, nil
		}
		return m, nil
	}

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "p":
		m.paused = !m.paused
		if m.OnPauseToggle != nil {
			m.OnPauseToggle()
		}
	case "c":
		if m.currentTask != nil && m.OnCancelTask != nil {
			m.OnCancelTask(m.currentTask.ID)
		}
		m.currentTask = nil
	case "D":
		m.showDetail = !m.showDetail
	}
	return m, nil
}

// View renders the full dashboard UI.
func (m WorkerModel) View() string {
	var b strings.Builder

	b.WriteString(renderHeader(m.connStatus, m.providers, m.width))
	b.WriteString("\n")
	b.WriteString(renderTaskQueue(m.taskQueue, m.width))
	b.WriteString("\n")

	if m.currentTask != nil {
		b.WriteString(renderCurrentTask(m.currentTask, m.showDetail, m.width))
		b.WriteString("\n")
	}

	if m.approval != nil {
		b.WriteString(renderApprovalDialog(*m.approval, m.width))
		b.WriteString("\n")
	}

	b.WriteString(renderCostFooter(m.costTracker, m.width))
	b.WriteString("\n")

	pauseLabel := "[p]ause"
	if m.paused {
		pauseLabel = "[p] PAUSED"
	}
	helpLine := fmt.Sprintf(" [q]uit  %s  [c]ancel  [D]etail  │ autopus worker", pauseLabel)
	b.WriteString(helpLine)

	return b.String()
}
