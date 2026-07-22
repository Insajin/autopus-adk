package orchestra

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandoffYieldPanes_DurableSession_RemovesTrackerOwnership(t *testing.T) {
	isolateSurfaceTracker(t)
	term := &yieldSessionTerminal{
		mockTerminal: &mockTerminal{name: "cmux"}, workspaceRef: "workspace:13",
	}
	ref := "surface:1414"
	trackSurfaceForTerminal(term, ref)
	session := OrchestraSession{
		ID: "yield-handoff-" + NewSessionID(), TerminalKind: "cmux",
		WorkspaceRef: "workspace:13", Panes: map[string]string{"claude": ref},
		CreatedAt: time.Now(),
	}
	require.NoError(t, SaveSession(session))
	t.Cleanup(func() { _ = RemoveSession(session.ID) })
	panes := []paneInfo{{paneID: terminal.PaneID(ref), provider: ProviderConfig{Name: "claude"}}}

	require.NoError(t, handoffYieldPanes(session.ID, term, panes))

	assert.NotContains(t, readTrackerRefs(surfaceTrackerFile(os.Getpid())), ref)
	_, err := LoadSession(session.ID)
	assert.NoError(t, err, "tracker handoff must leave the durable session intact")
}

func TestHandoffYieldPanes_UntrackFailure_ReportsDurableRecoveryHandle(t *testing.T) {
	original := yieldSurfaceUntracker
	yieldSurfaceUntracker = func(terminal.Terminal, string) error {
		return errors.New("tracker write failed")
	}
	t.Cleanup(func() { yieldSurfaceUntracker = original })
	session := OrchestraSession{
		ID:    "yield-handoff-failure-" + NewSessionID(),
		Panes: map[string]string{"claude": "surface:1414"}, CreatedAt: time.Now(),
	}
	require.NoError(t, SaveSession(session))
	t.Cleanup(func() { _ = RemoveSession(session.ID) })
	panes := []paneInfo{{paneID: "surface:1414", provider: ProviderConfig{Name: "claude"}}}

	err := handoffYieldPanes(session.ID, &mockTerminal{name: "cmux"}, panes)

	require.Error(t, err)
	assert.ErrorContains(t, err, session.ID)
	assert.ErrorContains(t, err, "auto orchestra cleanup --session-id "+session.ID)
	_, loadErr := LoadSession(session.ID)
	assert.NoError(t, loadErr, "failed tracker handoff must not consume the persisted recovery handle")
}
