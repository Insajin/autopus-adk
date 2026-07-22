package orchestra

import (
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type yieldSessionTerminal struct {
	*mockTerminal
	workspaceRef string
}

func (t *yieldSessionTerminal) WorkspaceRef() (string, error) {
	return t.workspaceRef, nil
}

func (t *yieldSessionTerminal) WithWorkspaceRef(ref string) (terminal.Terminal, error) {
	return &yieldSessionTerminal{mockTerminal: t.mockTerminal, workspaceRef: ref}, nil
}

func TestBuildYieldSession_PreservesProvidersInSingleRound(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.July, 12, 12, 0, 0, 0, time.UTC)
	panes := []paneInfo{
		{paneID: "pane-claude", provider: ProviderConfig{Name: "claude", Binary: "claude"}},
		{paneID: "pane-codex", provider: ProviderConfig{Name: "codex", Binary: "codex"}},
		{paneID: "pane-gemini", provider: ProviderConfig{Name: "gemini", Binary: "gemini"}},
	}
	responses := []ProviderResponse{
		{Provider: "claude", Output: "claude answer", Duration: 3 * time.Second},
		{Provider: "codex", Output: "codex answer", Duration: 4 * time.Second},
		{Provider: "gemini", Output: "gemini answer", Duration: 5 * time.Second},
	}

	term := &yieldSessionTerminal{
		mockTerminal: &mockTerminal{name: "cmux"},
		workspaceRef: "80187A1A-6ED8-4640-9A2A-F3235FF3F6D8",
	}
	session, err := buildYieldSession("orch-test", term, panes, responses, createdAt)
	require.NoError(t, err)

	assert.Equal(t, "orch-test", session.ID)
	assert.Equal(t, "cmux", session.TerminalKind)
	assert.Equal(t, "80187A1A-6ED8-4640-9A2A-F3235FF3F6D8", session.WorkspaceRef)
	assert.Equal(t, createdAt, session.CreatedAt)
	assert.Equal(t, map[string]string{
		"claude": "pane-claude",
		"codex":  "pane-codex",
		"gemini": "pane-gemini",
	}, session.Panes)
	require.Len(t, session.Providers, 3)
	assert.Equal(t, []SessionProviderConfig{
		{Name: "claude", Binary: "claude"},
		{Name: "codex", Binary: "codex"},
		{Name: "gemini", Binary: "gemini"},
	}, session.Providers)
	require.Len(t, session.Rounds, 1, "one provider fan-out must remain one debate round")
	require.Len(t, session.Rounds[0], 3)
	assert.Equal(t, "claude", session.Rounds[0][0].Provider)
	assert.Equal(t, "codex", session.Rounds[0][1].Provider)
	assert.Equal(t, "gemini", session.Rounds[0][2].Provider)
}

func TestBuildYieldSession_InvalidCmuxWorkspaceFailsClosed(t *testing.T) {
	term := &yieldSessionTerminal{
		mockTerminal: &mockTerminal{name: "cmux"},
		workspaceRef: "../unsafe",
	}

	_, err := buildYieldSession("orch-invalid", term, nil, nil, time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "validate cmux workspace context")
}
