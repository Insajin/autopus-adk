package orchestra

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// mockTerminal implements terminal.Terminal for testing pane runner logic.
type mockTerminal struct {
	mu                  sync.Mutex
	name                string
	splitPaneErr        error
	sendCommandErr      error
	sendCommandErrAfter int // error only after N successful calls (0 = always error)
	closeErr            error
	splitPaneCalls      []terminal.Direction
	sendCommandCalls    []struct {
		PaneID terminal.PaneID
		Cmd    string
	}
	closeCalls         []string
	nextPaneID         int
	createdPanes       []terminal.PaneID
	readScreenOutput   string   // configurable ReadScreen return value
	readScreenOutputs  []string // optional per-pane sequence for ReadScreen
	readScreenCalls    int      // count ReadScreen calls
	readScreenErr      error    // configurable ReadScreen error
	pipePaneStartCalls int      // count PipePaneStart calls
	pipePaneStopCalls  int      // count PipePaneStop calls
	pipePaneStartFiles []string // output files passed to PipePaneStart
	sendLongTextCalls  []struct {
		PaneID terminal.PaneID
		Text   string
	}
	autoComplete    bool
	mockOutput      string
	readIndexByPane map[terminal.PaneID]int
}

func (m *mockTerminal) Name() string { return m.name }

func (m *mockTerminal) CreateWorkspace(_ context.Context, _ string) error {
	return nil
}

func (m *mockTerminal) SplitPane(_ context.Context, dir terminal.Direction) (terminal.PaneID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.splitPaneCalls = append(m.splitPaneCalls, dir)
	if m.splitPaneErr != nil {
		return "", m.splitPaneErr
	}
	m.nextPaneID++
	id := terminal.PaneID(fmt.Sprintf("pane-%d", m.nextPaneID))
	m.createdPanes = append(m.createdPanes, id)
	return id, nil
}

func (m *mockTerminal) SendCommand(_ context.Context, paneID terminal.PaneID, cmd string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendCommandCalls = append(m.sendCommandCalls, struct {
		PaneID terminal.PaneID
		Cmd    string
	}{paneID, cmd})
	// If sendCommandErrAfter is set, only error after that many calls
	if m.sendCommandErrAfter > 0 && len(m.sendCommandCalls) <= m.sendCommandErrAfter {
		return nil
	}
	if m.sendCommandErr != nil {
		return m.sendCommandErr
	}
	if m.autoComplete {
		m.writeSentinelOutput(cmd)
	}
	return nil
}

func (m *mockTerminal) SendLongText(_ context.Context, paneID terminal.PaneID, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendLongTextCalls = append(m.sendLongTextCalls, struct {
		PaneID terminal.PaneID
		Text   string
	}{paneID, text})
	return nil
}

func (m *mockTerminal) Notify(_ context.Context, _ string) error {
	return nil
}

func (m *mockTerminal) ReadScreen(_ context.Context, paneID terminal.PaneID, _ terminal.ReadScreenOpts) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readScreenCalls++
	if len(m.readScreenOutputs) > 0 {
		if m.readIndexByPane == nil {
			m.readIndexByPane = make(map[terminal.PaneID]int)
		}
		m.readIndexByPane[paneID]++
		idx := (m.readIndexByPane[paneID] - 1) % len(m.readScreenOutputs)
		return m.readScreenOutputs[idx], m.readScreenErr
	}
	return m.readScreenOutput, m.readScreenErr
}

func (m *mockTerminal) PipePaneStart(_ context.Context, _ terminal.PaneID, outputFile string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pipePaneStartCalls++
	m.pipePaneStartFiles = append(m.pipePaneStartFiles, outputFile)
	return nil
}

func (m *mockTerminal) PipePaneStop(_ context.Context, _ terminal.PaneID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pipePaneStopCalls++
	return nil
}

func (m *mockTerminal) Close(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCalls = append(m.closeCalls, name)
	return m.closeErr
}

// pipePaneErrorMock embeds mockTerminal but overrides PipePaneStart to return an error.
type pipePaneErrorMock struct {
	mockTerminal
}

func (m *pipePaneErrorMock) PipePaneStart(_ context.Context, _ terminal.PaneID, _ string) error {
	return fmt.Errorf("pipe-pane start error")
}

// sendLongTextErrorMock embeds mockTerminal but overrides SendLongText to return an error.
type sendLongTextErrorMock struct {
	mockTerminal
}

func (m *sendLongTextErrorMock) SendLongText(_ context.Context, _ terminal.PaneID, _ string) error {
	return fmt.Errorf("send long text error")
}

// countingScreenMock embeds mockTerminal but alternates ReadScreen output
// based on call count to simulate screen changes between rounds.
type countingScreenMock struct {
	mockTerminal
	callCountByPane map[terminal.PaneID]int
	outputs         []string
}

func (m *countingScreenMock) ReadScreen(_ context.Context, paneID terminal.PaneID, _ terminal.ReadScreenOpts) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readScreenCalls++
	if m.callCountByPane == nil {
		m.callCountByPane = make(map[terminal.PaneID]int)
	}
	m.callCountByPane[paneID]++
	if len(m.outputs) == 0 {
		return m.readScreenOutput, m.readScreenErr
	}
	idx := (m.callCountByPane[paneID] - 1) % len(m.outputs)
	return m.outputs[idx], nil
}

func newCmuxMock() *mockTerminal {
	return &mockTerminal{name: "cmux", autoComplete: true, mockOutput: "mock output"}
}

func newPlainMock() *mockTerminal {
	return &mockTerminal{name: "plain"}
}

var outputFilePattern = regexp.MustCompile(`tee\s+([^;]+?)\s*;\s*echo\s+__AUTOPUS_DONE__\s*>>\s*(\S+)`)

func (m *mockTerminal) writeSentinelOutput(cmd string) {
	matches := outputFilePattern.FindStringSubmatch(cmd)
	if len(matches) != 3 {
		return
	}

	path := shellUnquote(matches[1])
	if path == "" {
		return
	}
	output := m.mockOutput
	if output == "" {
		output = "mock output"
	}
	_ = os.WriteFile(path, []byte(output+"\n"+sentinel+"\n"), 0o600)
}

func shellUnquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		s = s[1 : len(s)-1]
		s = strings.ReplaceAll(s, `'\''`, `'`)
	}
	return s
}
