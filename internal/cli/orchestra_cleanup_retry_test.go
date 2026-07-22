package cli

import (
	"errors"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func isolateCleanupPersistenceSeams(t *testing.T) {
	t.Helper()
	originalUpdate := orchestraSessionUpdater
	originalRemove := orchestraSessionRemover
	t.Cleanup(func() {
		orchestraSessionUpdater = originalUpdate
		orchestraSessionRemover = originalRemove
	})
}

func TestRunOrchestraCleanup_PartialUpdateFailure_RetryEventuallyRemoves(t *testing.T) {
	isolateCleanupPersistenceSeams(t)
	term := &cleanupTerminal{
		stubTerminal: stubTerminal{name: "cmux"}, workspaceRef: "workspace:99",
		closeErrors: map[string]error{"surface:1415": errors.New("pane busy")},
	}
	useCleanupTerminal(t, term)
	session := orchestra.OrchestraSession{
		ID: "cleanup-update-retry-" + orchestra.NewSessionID(), TerminalKind: "cmux",
		WorkspaceRef: "workspace:13",
		Panes:        map[string]string{"claude": "surface:1414", "codex": "surface:1415"},
		CreatedAt:    time.Now(),
	}
	require.NoError(t, orchestra.SaveSession(session))
	t.Cleanup(func() { _ = orchestra.RemoveSession(session.ID) })
	updateCalls := 0
	orchestraSessionUpdater = func(updated orchestra.OrchestraSession) error {
		updateCalls++
		if updateCalls == 1 {
			return errors.New("injected update failure")
		}
		return orchestra.UpdateSession(updated)
	}

	firstErr := runOrchestraCleanup(t.Context(), session.ID)

	require.Error(t, firstErr)
	loaded, loadErr := orchestra.LoadSession(session.ID)
	require.NoError(t, loadErr)
	assert.Equal(t, session.Panes, loaded.Panes,
		"atomic update failure must preserve the original retry set")
	delete(term.closeErrors, "surface:1415")
	require.NoError(t, runOrchestraCleanup(t.Context(), session.ID))
	assert.Equal(t, 2, countString(term.closed, "surface:1414"),
		"retry may safely close an already-absent pane")
	_, loadErr = orchestra.LoadSession(session.ID)
	assert.Error(t, loadErr)
}

func TestRunOrchestraCleanup_AllClosedRemoveFailure_RetrySkipsClose(t *testing.T) {
	isolateCleanupPersistenceSeams(t)
	term := &cleanupTerminal{
		stubTerminal: stubTerminal{name: "cmux"}, workspaceRef: "workspace:99",
		closeErrors: map[string]error{},
	}
	useCleanupTerminal(t, term)
	session := orchestra.OrchestraSession{
		ID: "cleanup-remove-retry-" + orchestra.NewSessionID(), TerminalKind: "cmux",
		WorkspaceRef: "workspace:13", Panes: map[string]string{"claude": "surface:1414"},
		CreatedAt: time.Now(),
	}
	require.NoError(t, orchestra.SaveSession(session))
	t.Cleanup(func() { _ = orchestra.RemoveSession(session.ID) })
	removeCalls := 0
	orchestraSessionRemover = func(id string) error {
		removeCalls++
		if removeCalls == 1 {
			return errors.New("injected remove failure")
		}
		return orchestra.RemoveSession(id)
	}

	firstErr := runOrchestraCleanup(t.Context(), session.ID)

	require.Error(t, firstErr)
	loaded, loadErr := orchestra.LoadSession(session.ID)
	require.NoError(t, loadErr)
	assert.Empty(t, loaded.Panes, "all-close must durably commit a tombstone before removal")
	closedBeforeRetry := len(term.closed)
	require.NoError(t, runOrchestraCleanup(t.Context(), session.ID))
	assert.Len(t, term.closed, closedBeforeRetry, "tombstone retry must not close panes again")
}

func TestRunOrchestraCleanup_EmptyTombstone_RemoveRetryAvoidsTerminal(t *testing.T) {
	isolateCleanupPersistenceSeams(t)
	session := orchestra.OrchestraSession{
		ID:    "cleanup-empty-retry-" + orchestra.NewSessionID(),
		Panes: map[string]string{}, CreatedAt: time.Now(),
	}
	require.NoError(t, orchestra.SaveSession(session))
	t.Cleanup(func() { _ = orchestra.RemoveSession(session.ID) })
	originalDetector := orchestraSessionTerminalDetector
	detectCalls := 0
	orchestraSessionTerminalDetector = func() terminal.Terminal {
		detectCalls++
		return &terminal.PlainAdapter{}
	}
	t.Cleanup(func() { orchestraSessionTerminalDetector = originalDetector })
	removeCalls := 0
	orchestraSessionRemover = func(id string) error {
		removeCalls++
		if removeCalls == 1 {
			return errors.New("injected remove failure")
		}
		return orchestra.RemoveSession(id)
	}

	require.Error(t, runOrchestraCleanup(t.Context(), session.ID))
	require.NoError(t, runOrchestraCleanup(t.Context(), session.ID))
	assert.Zero(t, detectCalls)
}
