package pipeline

import (
	"context"
	"fmt"
	"sync"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// teamMockTerminal is a test double that tracks all terminal operations.
type teamMockTerminal struct {
	mu             sync.Mutex
	name           string
	splitCount     int
	splitDirs      []terminal.Direction
	sentCommands   []sentCmd
	closedSessions []string
	closedPanes    []string

	// Configuration for error simulation.
	failSplitAfter int // -1 means never fail
	failCloseErr   error
}

type sentCmd struct {
	paneID terminal.PaneID
	cmd    string
}

func newTeamMock(name string) *teamMockTerminal {
	return &teamMockTerminal{
		name:           name,
		failSplitAfter: -1,
	}
}

func (m *teamMockTerminal) Name() string { return m.name }

func (m *teamMockTerminal) CreateWorkspace(_ context.Context, _ string) error {
	return nil
}

func (m *teamMockTerminal) SplitPane(_ context.Context, dir terminal.Direction) (terminal.PaneID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.splitCount++
	m.splitDirs = append(m.splitDirs, dir)
	if m.failSplitAfter >= 0 && m.splitCount > m.failSplitAfter {
		return "", fmt.Errorf("split pane failure at split %d", m.splitCount)
	}
	return terminal.PaneID(fmt.Sprintf("pane-%d", m.splitCount)), nil
}

func (m *teamMockTerminal) SendCommand(_ context.Context, paneID terminal.PaneID, cmd string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentCommands = append(m.sentCommands, sentCmd{paneID: paneID, cmd: cmd})
	return nil
}

func (m *teamMockTerminal) Notify(_ context.Context, _ string) error { return nil }

func (m *teamMockTerminal) Close(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closedSessions = append(m.closedSessions, name)
	if m.failCloseErr != nil {
		return m.failCloseErr
	}
	return nil
}

func (m *teamMockTerminal) ReadScreen(_ context.Context, _ terminal.PaneID, _ terminal.ReadScreenOpts) (string, error) {
	return "", nil
}
func (m *teamMockTerminal) PipePaneStart(_ context.Context, _ terminal.PaneID, _ string) error {
	return nil
}
func (m *teamMockTerminal) PipePaneStop(_ context.Context, _ terminal.PaneID) error { return nil }
