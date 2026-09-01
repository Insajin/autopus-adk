package orchestra

import (
	"context"
	"sync"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// seqScreenMock returns a scripted sequence of ReadScreen outputs, sticking on
// the final element once exhausted (unlike mockTerminal's modulo cycling).
// It records SendLongText/SendCommand calls so prompt-gating can be asserted.
type seqScreenMock struct {
	mu        sync.Mutex
	name      string
	screens   []string
	idx       int
	longTexts []string
	commands  []string
	readCalls int
	splitErr  error
}

func (m *seqScreenMock) Name() string                                  { return m.name }
func (m *seqScreenMock) CreateWorkspace(context.Context, string) error { return nil }

func (m *seqScreenMock) SplitPane(context.Context, terminal.Direction) (terminal.PaneID, error) {
	if m.splitErr != nil {
		return "", m.splitErr
	}
	return terminal.PaneID("pane-1"), nil
}

func (m *seqScreenMock) SendCommand(_ context.Context, _ terminal.PaneID, cmd string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commands = append(m.commands, cmd)
	return nil
}

func (m *seqScreenMock) SendLongText(_ context.Context, _ terminal.PaneID, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.longTexts = append(m.longTexts, text)
	return nil
}

func (m *seqScreenMock) Notify(context.Context, string) error { return nil }

func (m *seqScreenMock) ReadScreen(context.Context, terminal.PaneID, terminal.ReadScreenOpts) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readCalls++
	if len(m.screens) == 0 {
		return "", nil
	}
	if m.idx >= len(m.screens) {
		return m.screens[len(m.screens)-1], nil
	}
	s := m.screens[m.idx]
	m.idx++
	return s, nil
}

func (m *seqScreenMock) PipePaneStart(context.Context, terminal.PaneID, string) error { return nil }
func (m *seqScreenMock) PipePaneStop(context.Context, terminal.PaneID) error          { return nil }
func (m *seqScreenMock) Close(context.Context, string) error                          { return nil }

func (m *seqScreenMock) longTextsSnapshot() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.longTexts))
	copy(out, m.longTexts)
	return out
}

// commandsSnapshot exists so an assertion can read sent commands while Execute
// is still running. Reading m.commands directly is only safe after Execute has
// returned, which forces a test to bound Execute by wall clock and makes the
// assertion race the scheduler.
func (m *seqScreenMock) commandsSnapshot() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.commands))
	copy(out, m.commands)
	return out
}
