package cli

import (
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOrchestraCleanupCmd_Flags(t *testing.T) {
	t.Parallel()

	cmd := newOrchestraCleanupCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "cleanup", cmd.Use)
	assert.NotNil(t, cmd.Flags().Lookup("session-id"), "session-id flag must exist")
}

func TestNewOrchestraCleanupCmd_RequiresSessionID(t *testing.T) {
	t.Parallel()

	cmd := newOrchestraCleanupCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.Error(t, err, "should fail without --session-id")
}

func TestRunOrchestraCleanup_MissingSession(t *testing.T) {
	t.Parallel()

	// Cleanup of a non-existent session should not error (idempotent).
	err := runOrchestraCleanup(t.Context(), "nonexistent-session-cleanup-test")
	assert.NoError(t, err)
}

func TestRunOrchestraCleanup_RemovesSessionFile(t *testing.T) {
	t.Parallel()

	// Create a session file
	session := orchestra.OrchestraSession{
		ID:        "test-cleanup-" + orchestra.NewSessionID(),
		Panes:     map[string]string{},
		CreatedAt: time.Now(),
	}
	require.NoError(t, orchestra.SaveSession(session))

	// Cleanup should succeed (pane kill will fail since no real terminal, but that's fine)
	err := runOrchestraCleanup(t.Context(), session.ID)
	assert.NoError(t, err)

	// Session file should be removed
	_, loadErr := orchestra.LoadSession(session.ID)
	assert.Error(t, loadErr, "session should be removed after cleanup")
}
