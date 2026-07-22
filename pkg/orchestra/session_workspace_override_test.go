package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSessionTerminalWithWorkspace_LegacyCmux_UsesExplicitOverride(t *testing.T) {
	detected := &sessionContextTerminal{
		mockTerminal: &mockTerminal{name: "cmux"}, workspaceRef: "workspace:99",
	}
	session := &OrchestraSession{Panes: map[string]string{"claude": "surface:1414"}}

	resolved, err := ResolveSessionTerminalWithWorkspace(session, detected, "workspace:13")

	require.NoError(t, err)
	contextual, ok := resolved.(interface{ WorkspaceRef() (string, error) })
	require.True(t, ok)
	workspaceRef, err := contextual.WorkspaceRef()
	require.NoError(t, err)
	assert.Equal(t, "workspace:13", workspaceRef)
}

func TestResolveSessionTerminalWithWorkspace_InvalidOverride_FailsClosed(t *testing.T) {
	session := &OrchestraSession{Panes: map[string]string{"claude": "surface:1414"}}

	_, err := ResolveSessionTerminalWithWorkspace(session, &mockTerminal{name: "cmux"}, "../unsafe")

	require.Error(t, err)
	assert.ErrorContains(t, err, "workspace override")
}

func TestResolveSessionTerminalWithWorkspace_StructuredSessionRejectsConflict(t *testing.T) {
	session := &OrchestraSession{
		TerminalKind: "cmux", WorkspaceRef: "workspace:13",
		Panes: map[string]string{"claude": "surface:1414"},
	}

	_, err := ResolveSessionTerminalWithWorkspace(session, &mockTerminal{name: "cmux"}, "workspace:21")

	require.Error(t, err)
	assert.ErrorContains(t, err, "legacy cmux")
}

func TestResolveSessionTerminalWithWorkspace_LegacyTmuxRejectsCmuxOverride(t *testing.T) {
	session := &OrchestraSession{Panes: map[string]string{"claude": "%42"}}

	_, err := ResolveSessionTerminalWithWorkspace(session, &mockTerminal{name: "tmux"}, "workspace:13")

	require.Error(t, err)
	assert.ErrorContains(t, err, "cmux pane")
}
