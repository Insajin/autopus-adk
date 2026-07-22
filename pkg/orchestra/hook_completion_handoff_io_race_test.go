package orchestra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type readyThenReadErrorTerminal struct {
	mockTerminal
	session *HookSession
}

func (t *readyThenReadErrorTerminal) ReadScreen(_ context.Context, _ terminal.PaneID, _ terminal.ReadScreenOpts) (string, error) {
	if err := t.session.writeArtifact(RoundSignalName("codex", 2, "ready"), nil, 0o600); err != nil {
		return "", err
	}
	return "", errors.New("screen I/O failed after next-ready")
}

func TestResolveHookCompletionHandoff_NextReadyWinsReadErrorRace(t *testing.T) {
	session, err := NewHookSession("handoff-read-error-race-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	provider := ProviderConfig{Name: "codex", Binary: "codex"}
	term := &readyThenReadErrorTerminal{
		mockTerminal: mockTerminal{name: "cmux"}, session: session,
	}
	cfg := OrchestraConfig{Providers: []ProviderConfig{provider}, Terminal: term, DebateRounds: 2}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	provenance, err := resolveHookCompletionHandoff(
		ctx, cfg, session, provider, paneInfo{provider: provider, paneID: "surface:1"}, 1,
	)

	require.NoError(t, err)
	assert.Equal(t, hookCompletionNextRoundReady, provenance)
	assert.True(t, session.HasHook("codex"))
}
