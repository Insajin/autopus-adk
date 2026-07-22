package orchestra

import (
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setTmuxServerForTest(t *testing.T, value string) string {
	t.Helper()
	t.Setenv("TMUX", value)
	serverRef, ok := currentTmuxServerRef()
	require.True(t, ok)
	require.NotEmpty(t, serverRef)
	return serverRef
}

func TestResolveSessionTerminal_TmuxMatchingServer_UsesDetectedTerminal(t *testing.T) {
	serverRef := setTmuxServerForTest(t, "/tmp/tmux-501/default,12345,0")
	detected := &mockTerminal{name: "tmux"}
	session := &OrchestraSession{
		TerminalKind: "tmux", TmuxServerRef: serverRef,
		Panes: map[string]string{"claude": "%42"},
	}

	resolved, err := ResolveSessionTerminal(session, detected)

	require.NoError(t, err)
	assert.Same(t, detected, resolved)
}

func TestResolveSessionTerminal_TmuxDifferentServer_FailsClosed(t *testing.T) {
	persistedRef := setTmuxServerForTest(t, "/tmp/tmux-501/default,12345,0")
	setTmuxServerForTest(t, "/tmp/tmux-501/other,77777,0")
	session := &OrchestraSession{
		TerminalKind: "tmux", TmuxServerRef: persistedRef,
		Panes: map[string]string{"claude": "%42"},
	}

	_, err := ResolveSessionTerminal(session, &mockTerminal{name: "tmux"})

	require.Error(t, err)
	assert.ErrorContains(t, err, "tmux server")
}

func TestResolveSessionTerminal_TmuxFromNonTmuxContext_FailsClosed(t *testing.T) {
	serverRef := setTmuxServerForTest(t, "/tmp/tmux-501/default,12345,0")
	session := &OrchestraSession{
		TerminalKind: "tmux", TmuxServerRef: serverRef,
		Panes: map[string]string{"claude": "%42"},
	}

	_, err := ResolveSessionTerminal(session, &terminal.PlainAdapter{})

	require.Error(t, err)
	assert.ErrorContains(t, err, "active tmux")
}

func TestResolveSessionTerminal_LegacyTmuxFromCmuxContext_FailsClosed(t *testing.T) {
	setTmuxServerForTest(t, "/tmp/tmux-501/default,12345,0")
	session := &OrchestraSession{Panes: map[string]string{"claude": "%42"}}

	_, err := ResolveSessionTerminal(session, &mockTerminal{name: "cmux"})

	require.Error(t, err)
	assert.ErrorContains(t, err, "legacy tmux")
}

func TestBuildYieldSession_Tmux_PersistsServerIdentity(t *testing.T) {
	serverRef := setTmuxServerForTest(t, "/tmp/tmux-501/default,12345,0")
	panes := []paneInfo{{paneID: "%42", provider: ProviderConfig{Name: "claude", Binary: "claude"}}}

	session, err := buildYieldSession(
		"orch-tmux", &mockTerminal{name: "tmux"}, panes, nil,
		time.Date(2026, time.July, 22, 0, 0, 0, 0, time.UTC),
	)

	require.NoError(t, err)
	assert.Equal(t, "tmux", session.TerminalKind)
	assert.Equal(t, serverRef, session.TmuxServerRef)
}

func TestSaveAndLoadSession_TmuxServerIdentity_RoundTrips(t *testing.T) {
	serverRef := setTmuxServerForTest(t, "/tmp/tmux-501/default,12345,0")
	session := OrchestraSession{
		ID: "tmux-roundtrip-" + NewSessionID(), TerminalKind: "tmux",
		TmuxServerRef: serverRef, Panes: map[string]string{"claude": "%42"},
		CreatedAt: time.Now(),
	}
	require.NoError(t, SaveSession(session))
	t.Cleanup(func() { _ = RemoveSession(session.ID) })

	loaded, err := LoadSession(session.ID)

	require.NoError(t, err)
	assert.Equal(t, serverRef, loaded.TmuxServerRef)
}
