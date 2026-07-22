package orchestra

import (
	"os"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionID_Format(t *testing.T) {
	t.Parallel()

	id := NewSessionID()
	assert.Contains(t, id, "orch-")
	assert.Greater(t, len(id), 15) // orch- + timestamp + hex
}

func TestNewSessionID_Unique(t *testing.T) {
	t.Parallel()

	id1 := NewSessionID()
	id2 := NewSessionID()
	assert.NotEqual(t, id1, id2)
}

func TestSaveAndLoadSession(t *testing.T) {
	t.Parallel()

	session := OrchestraSession{
		ID:           "test-save-load-" + NewSessionID(),
		TerminalKind: "cmux",
		WorkspaceRef: "workspace:17",
		Panes:        map[string]string{"claude": "surface:1", "gemini": "surface:2"},
		Providers: []SessionProviderConfig{
			{Name: "claude", Binary: "claude"},
			{Name: "gemini", Binary: "gemini"},
		},
		Rounds: [][]SessionProviderResponse{
			{
				{Provider: "claude", Output: "hello", DurationMs: 100, TimedOut: false},
				{Provider: "gemini", Output: "world", DurationMs: 200, TimedOut: false},
			},
		},
		CreatedAt: time.Now().Truncate(time.Second),
	}

	err := SaveSession(session)
	require.NoError(t, err)
	defer func() { _ = RemoveSession(session.ID) }()

	loaded, err := LoadSession(session.ID)
	require.NoError(t, err)

	assert.Equal(t, session.ID, loaded.ID)
	assert.Equal(t, "cmux", loaded.TerminalKind)
	assert.Equal(t, "workspace:17", loaded.WorkspaceRef)
	assert.Equal(t, session.Panes, loaded.Panes)
	assert.Len(t, loaded.Providers, 2)
	assert.Equal(t, "claude", loaded.Providers[0].Name)
	assert.Len(t, loaded.Rounds, 1)
	assert.Len(t, loaded.Rounds[0], 2)
	assert.Equal(t, "hello", loaded.Rounds[0][0].Output)
}

type sessionContextTerminal struct {
	*mockTerminal
	workspaceRef string
}

func (t *sessionContextTerminal) WorkspaceRef() (string, error) {
	return t.workspaceRef, nil
}

func (t *sessionContextTerminal) WithWorkspaceRef(ref string) (terminal.Terminal, error) {
	return &sessionContextTerminal{mockTerminal: t.mockTerminal, workspaceRef: ref}, nil
}

func TestResolveSessionTerminal_CmuxContext_RestoresPersistedWorkspace(t *testing.T) {
	detected := &sessionContextTerminal{
		mockTerminal: &mockTerminal{name: "cmux"},
		workspaceRef: "workspace:99",
	}
	session := &OrchestraSession{
		ID:           "context-restore",
		TerminalKind: "cmux",
		WorkspaceRef: "workspace:13",
		Panes:        map[string]string{"claude": "surface:1"},
	}

	resolved, err := ResolveSessionTerminal(session, detected)
	require.NoError(t, err)
	contextual, ok := resolved.(interface{ WorkspaceRef() (string, error) })
	require.True(t, ok)
	got, err := contextual.WorkspaceRef()
	require.NoError(t, err)
	assert.Equal(t, "workspace:13", got)
	assert.Equal(t, "workspace:99", detected.workspaceRef,
		"restoring a session must not mutate the caller's terminal context")
}

func TestResolveSessionTerminal_LegacyCmuxWithoutWorkspace_FailsClosed(t *testing.T) {
	detected := &sessionContextTerminal{
		mockTerminal: &mockTerminal{name: "cmux"},
		workspaceRef: "workspace:unproven",
	}
	session := &OrchestraSession{
		ID:    "legacy-cmux",
		Panes: map[string]string{"claude": "surface:1"},
	}

	_, err := ResolveSessionTerminal(session, detected)

	require.Error(t, err)
	assert.ErrorContains(t, err, "legacy session")
	assert.ErrorContains(t, err, "workspace")
}

func TestResolveSessionTerminal_LegacyGlobalTmuxPanes_UsesExplicitSafeCapability(t *testing.T) {
	detected := &mockTerminal{name: "tmux"}
	session := &OrchestraSession{
		ID:    "legacy-tmux",
		Panes: map[string]string{"claude": "%42"},
	}

	resolved, err := ResolveSessionTerminal(session, detected)

	require.NoError(t, err)
	assert.Same(t, detected, resolved)
}

func TestResolveSessionTerminal_LegacyGlobalTmuxPanes_FromPlainShellFailsClosed(t *testing.T) {
	session := &OrchestraSession{
		ID:    "legacy-tmux-detached",
		Panes: map[string]string{"claude": "%42"},
	}

	_, err := ResolveSessionTerminal(session, &terminal.PlainAdapter{})

	require.Error(t, err)
	assert.ErrorContains(t, err, "active tmux")
}

func TestResolveSessionTerminal_PersistedCmuxFromPlainShell_ReconstructsBackend(t *testing.T) {
	session := &OrchestraSession{
		ID:           "detached-cmux",
		TerminalKind: "cmux",
		WorkspaceRef: "workspace:13",
		Panes:        map[string]string{"claude": "surface:1414"},
	}

	resolved, err := ResolveSessionTerminal(session, &terminal.PlainAdapter{})

	require.NoError(t, err)
	assert.Equal(t, "cmux", resolved.Name())
	contextual, ok := resolved.(terminal.WorkspaceContextProvider)
	require.True(t, ok)
	workspaceRef, err := contextual.WorkspaceRef()
	require.NoError(t, err)
	assert.Equal(t, "workspace:13", workspaceRef)
}

func TestResolveSessionTerminal_PersistedTmux_DoesNotDowngradeToDetectedCmux(t *testing.T) {
	detected := &sessionContextTerminal{
		mockTerminal: &mockTerminal{name: "cmux"},
		workspaceRef: "workspace:13",
	}
	session := &OrchestraSession{
		ID:           "detached-tmux",
		TerminalKind: "tmux",
		Panes:        map[string]string{"claude": "%42"},
	}

	_, err := ResolveSessionTerminal(session, detected)

	require.Error(t, err)
	assert.ErrorContains(t, err, "active tmux")
}

func TestResolveSessionTerminal_IncompatiblePaneReferences_FailBeforeBackendUse(t *testing.T) {
	tests := []struct {
		name    string
		session *OrchestraSession
	}{
		{
			name: "cmux with tmux pane",
			session: &OrchestraSession{
				TerminalKind: "cmux", WorkspaceRef: "workspace:13",
				Panes: map[string]string{"claude": "%42"},
			},
		},
		{
			name: "tmux with cmux surface",
			session: &OrchestraSession{
				TerminalKind: "tmux", Panes: map[string]string{"claude": "surface:1414"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ResolveSessionTerminal(test.session, &terminal.PlainAdapter{})
			require.Error(t, err)
			assert.ErrorContains(t, err, "incompatible pane reference")
		})
	}
}

func TestResolveSessionTerminal_InvalidCmuxWorkspace_FailsBeforeClone(t *testing.T) {
	detected := &sessionContextTerminal{
		mockTerminal: &mockTerminal{name: "cmux"}, workspaceRef: "workspace:13",
	}
	session := &OrchestraSession{
		TerminalKind: "cmux", WorkspaceRef: "../wrong-workspace",
		Panes: map[string]string{"claude": "surface:1414"},
	}

	_, err := ResolveSessionTerminal(session, detected)

	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid persisted cmux workspace")
	assert.Equal(t, "workspace:13", detected.workspaceRef)
}

func TestLoadSession_NotFound(t *testing.T) {
	t.Parallel()

	_, err := LoadSession("nonexistent-session-id-12345")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read session")
}

func TestSaveSession_Permissions(t *testing.T) {
	t.Parallel()

	session := OrchestraSession{
		ID:        "test-perms-" + NewSessionID(),
		Panes:     map[string]string{},
		CreatedAt: time.Now(),
	}

	require.NoError(t, SaveSession(session))
	defer func() { _ = RemoveSession(session.ID) }()

	info, err := os.Stat(sessionFilePath(session.ID))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "session file must have 0600 permissions")
}

func TestRemoveSession_Idempotent(t *testing.T) {
	t.Parallel()

	// Removing a non-existent session should return nil (idempotent).
	err := RemoveSession("nonexistent-cleanup-id-67890")
	assert.NoError(t, err)
}

func TestRemoveSession_AfterSave(t *testing.T) {
	t.Parallel()

	session := OrchestraSession{
		ID:        "test-remove-" + NewSessionID(),
		Panes:     map[string]string{},
		CreatedAt: time.Now(),
	}

	require.NoError(t, SaveSession(session))

	// Should load successfully
	_, err := LoadSession(session.ID)
	require.NoError(t, err)

	// Remove
	require.NoError(t, RemoveSession(session.ID))

	// Should fail to load after removal
	_, err = LoadSession(session.ID)
	assert.Error(t, err)
}
