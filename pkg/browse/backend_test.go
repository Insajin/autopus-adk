// Package browse provides browser automation backends for different terminal environments.
package browse

import (
	"context"
	"testing"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// mockTerminal implements terminal.Terminal for testing factory routing.
type mockTerminal struct {
	name string
}

func (m *mockTerminal) Name() string { return m.name }
func (m *mockTerminal) CreateWorkspace(_ context.Context, _ string) error {
	return nil
}
func (m *mockTerminal) SplitPane(_ context.Context, _ terminal.Direction) (terminal.PaneID, error) {
	return "", nil
}
func (m *mockTerminal) SendCommand(_ context.Context, _ terminal.PaneID, _ string) error {
	return nil
}
func (m *mockTerminal) Notify(_ context.Context, _ string) error { return nil }
func (m *mockTerminal) Close(_ context.Context, _ string) error  { return nil }
func (m *mockTerminal) ReadScreen(_ context.Context, _ terminal.PaneID, _ terminal.ReadScreenOpts) (string, error) {
	return "", nil
}
func (m *mockTerminal) PipePaneStart(_ context.Context, _ terminal.PaneID, _ string) error {
	return nil
}
func (m *mockTerminal) PipePaneStop(_ context.Context, _ terminal.PaneID) error { return nil }

// TestNewBackend_CmuxTerminal_ReturnsCmuxBackend verifies that a cmux terminal
// produces a CmuxBrowserBackend from the factory.
func TestNewBackend_CmuxTerminal_ReturnsCmuxBackend(t *testing.T) {
	term := &mockTerminal{name: "cmux"}
	backend := NewBackend(term)
	if backend.Name() != "cmux" {
		t.Errorf("expected cmux backend, got %q", backend.Name())
	}
	if _, ok := backend.(*CmuxBrowserBackend); !ok {
		t.Errorf("expected *CmuxBrowserBackend, got %T", backend)
	}
}

// TestNewBackend_TmuxTerminal_ReturnsAgentBackend verifies that a tmux terminal
// produces an AgentBrowserBackend from the factory.
func TestNewBackend_TmuxTerminal_ReturnsAgentBackend(t *testing.T) {
	term := &mockTerminal{name: "tmux"}
	backend := NewBackend(term)
	if backend.Name() != "agent-browser" {
		t.Errorf("expected agent-browser backend, got %q", backend.Name())
	}
	if _, ok := backend.(*AgentBrowserBackend); !ok {
		t.Errorf("expected *AgentBrowserBackend, got %T", backend)
	}
}

// TestNewBackend_PlainTerminal_ReturnsAgentBackend verifies that a plain terminal
// produces an AgentBrowserBackend from the factory.
func TestNewBackend_PlainTerminal_ReturnsAgentBackend(t *testing.T) {
	term := &mockTerminal{name: "plain"}
	backend := NewBackend(term)
	if backend.Name() != "agent-browser" {
		t.Errorf("expected agent-browser backend, got %q", backend.Name())
	}
	if _, ok := backend.(*AgentBrowserBackend); !ok {
		t.Errorf("expected *AgentBrowserBackend, got %T", backend)
	}
}

// TestNewBackend_NilTerminal_ReturnsAgentBackend verifies that a nil terminal
// produces an AgentBrowserBackend as the safe default.
func TestNewBackend_NilTerminal_ReturnsAgentBackend(t *testing.T) {
	backend := NewBackend(nil)
	if backend.Name() != "agent-browser" {
		t.Errorf("expected agent-browser backend, got %q", backend.Name())
	}
	if _, ok := backend.(*AgentBrowserBackend); !ok {
		t.Errorf("expected *AgentBrowserBackend, got %T", backend)
	}
}
