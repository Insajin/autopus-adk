package orchestra

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const committedPaneID terminal.PaneID = "pane-committed"
const paneCommitWorkspaceRef = "workspace:13"

// paneCommitTerminal models a pane that was provisioned successfully and then
// fails at a selected post-provisioning I/O boundary.
type paneCommitTerminal struct {
	splitCalls        int
	splitID           terminal.PaneID
	splitErr          error
	sendLongTextCalls int
	sendCommandCalls  int
	longTextErrAt     int
	commandErrAt      int
	screen            string
	readErr           error
	closeErr          error
	closed            []string
	workspaceRef      string
}

func (m *paneCommitTerminal) Name() string { return "cmux" }

func (m *paneCommitTerminal) WorkspaceRef() (string, error) {
	ref := m.workspaceRef
	if ref == "" {
		ref = paneCommitWorkspaceRef
	}
	if err := validatePaneCommitWorkspaceRef(ref); err != nil {
		return "", err
	}
	return ref, nil
}

func (m *paneCommitTerminal) WithWorkspaceRef(ref string) (terminal.Terminal, error) {
	if err := validatePaneCommitWorkspaceRef(ref); err != nil {
		return nil, err
	}
	clone := *m
	clone.workspaceRef = ref
	clone.closed = append([]string(nil), m.closed...)
	return &clone, nil
}

func validatePaneCommitWorkspaceRef(ref string) error {
	if _, err := terminal.NewCmuxAdapterWithWorkspace(ref); err != nil {
		return fmt.Errorf("invalid pane fixture workspace: %w", err)
	}
	if ref != paneCommitWorkspaceRef {
		return fmt.Errorf("pane fixture workspace %q is not %q", ref, paneCommitWorkspaceRef)
	}
	return nil
}

func TestPaneCommitTerminalWorkspaceContext_ValidatedClone(t *testing.T) {
	original := &paneCommitTerminal{closed: []string{"surface:1"}}

	workspaceRef, err := original.WorkspaceRef()
	require.NoError(t, err)
	assert.Equal(t, paneCommitWorkspaceRef, workspaceRef)

	restored, err := original.WithWorkspaceRef(paneCommitWorkspaceRef)
	require.NoError(t, err)
	clone, ok := restored.(*paneCommitTerminal)
	require.True(t, ok)
	assert.NotSame(t, original, clone)
	assert.Empty(t, original.workspaceRef)
	assert.Equal(t, paneCommitWorkspaceRef, clone.workspaceRef)
	clone.closed[0] = "surface:2"
	assert.Equal(t, []string{"surface:1"}, original.closed)

	_, err = original.WithWorkspaceRef("workspace:14")
	assert.Error(t, err)
	_, err = original.WithWorkspaceRef("../unsafe")
	assert.Error(t, err)
}

func (m *paneCommitTerminal) CreateWorkspace(context.Context, string) error { return nil }

func (m *paneCommitTerminal) SplitPane(context.Context, terminal.Direction) (terminal.PaneID, error) {
	m.splitCalls++
	paneID := m.splitID
	if paneID == "" && m.splitErr == nil {
		paneID = committedPaneID
	}
	return paneID, m.splitErr
}

func (m *paneCommitTerminal) SendCommand(_ context.Context, _ terminal.PaneID, _ string) error {
	m.sendCommandCalls++
	if m.commandErrAt > 0 && m.sendCommandCalls == m.commandErrAt {
		return errors.New("injected SendCommand failure")
	}
	return nil
}

func (m *paneCommitTerminal) SendLongText(_ context.Context, _ terminal.PaneID, _ string) error {
	m.sendLongTextCalls++
	if m.longTextErrAt > 0 && m.sendLongTextCalls == m.longTextErrAt {
		return errors.New("injected SendLongText failure")
	}
	return nil
}

func (m *paneCommitTerminal) Notify(context.Context, string) error { return nil }

func (m *paneCommitTerminal) ReadScreen(context.Context, terminal.PaneID, terminal.ReadScreenOpts) (string, error) {
	return m.screen, m.readErr
}

func (m *paneCommitTerminal) PipePaneStart(context.Context, terminal.PaneID, string) error {
	return nil
}

func (m *paneCommitTerminal) PipePaneStop(context.Context, terminal.PaneID) error { return nil }

func (m *paneCommitTerminal) Close(_ context.Context, ref string) error {
	m.closed = append(m.closed, ref)
	return m.closeErr
}
