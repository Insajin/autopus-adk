package orchestra

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInteractivePaneBackend_ResetsHookAttemptBeforeProviderLaunch(t *testing.T) {
	sessionID := "pane-attempt-reset-" + NewSessionID()
	session, err := NewHookSession(sessionID)
	require.NoError(t, err)
	defer session.Cleanup()

	stale := []string{
		"codex-done",
		"codex-result.json",
		RoundSignalName("codex", 0, "ready"),
	}
	writeAttemptFiles(t, session, stale)

	term := &sendLongTextErrorMock{mockTerminal: *newCmuxMock()}
	backend := NewInteractivePaneBackend(OrchestraConfig{
		Terminal:   term,
		HookMode:   true,
		SessionID:  sessionID,
		Providers:  []ProviderConfig{{Name: "codex", Binary: "codex"}},
		WorkingDir: t.TempDir(),
	})

	response, execErr := backend.Execute(context.Background(), ProviderRequest{
		Provider: "codex",
		Role:     "reviewer",
		Config:   ProviderConfig{Name: "codex", Binary: "codex"},
	})
	require.Error(t, execErr)
	require.NotNil(t, response)
	assert.Equal(t, paneBackendName, response.ExecutedBackend)
	assertAttemptFilesMissing(t, session, stale)
	assert.Empty(t, term.sendLongTextCalls, "provider launch failed at the transport seam after reset")
}
