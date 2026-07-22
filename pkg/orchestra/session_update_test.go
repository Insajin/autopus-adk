package orchestra

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateSession_ReplacesPaneSetAtomicallyAndPreservesPermissions(t *testing.T) {
	session := OrchestraSession{
		ID:           "update-panes-" + NewSessionID(),
		TerminalKind: "cmux",
		WorkspaceRef: "workspace:13",
		Panes: map[string]string{
			"claude": "surface:1414",
			"codex":  "surface:1415",
		},
		CreatedAt: time.Now(),
	}
	require.NoError(t, SaveSession(session))
	t.Cleanup(func() { _ = RemoveSession(session.ID) })
	session.Panes = map[string]string{"codex": "surface:1415"}

	require.NoError(t, UpdateSession(session))

	loaded, err := LoadSession(session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.Panes, loaded.Panes)
	info, err := os.Stat(sessionFilePath(session.ID))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestUpdateSession_CommitFailurePreservesOriginalRetryHandle(t *testing.T) {
	session := OrchestraSession{
		ID:           "update-failure-" + NewSessionID(),
		TerminalKind: "cmux",
		WorkspaceRef: "workspace:13",
		Panes:        map[string]string{"claude": "surface:1414", "codex": "surface:1415"},
		CreatedAt:    time.Now(),
	}
	require.NoError(t, SaveSession(session))
	t.Cleanup(func() { _ = RemoveSession(session.ID) })
	updated := session
	updated.Panes = map[string]string{"codex": "surface:1415"}
	injected := errors.New("injected atomic rename failure")

	err := updateSessionWithCommit(updated, func(*os.Root, string, string) error {
		return injected
	})

	require.ErrorIs(t, err, injected)
	loaded, loadErr := LoadSession(session.ID)
	require.NoError(t, loadErr)
	assert.Equal(t, session.Panes, loaded.Panes,
		"a failed replacement must leave the original retry handle intact")
}

func TestUpdateSession_MissingTargetDoesNotCreateSession(t *testing.T) {
	session := OrchestraSession{
		ID:    "update-missing-" + NewSessionID(),
		Panes: map[string]string{"claude": "surface:1"},
	}

	err := UpdateSession(session)

	require.Error(t, err)
	_, loadErr := LoadSession(session.ID)
	assert.Error(t, loadErr)
}

func TestUpdateSession_LegacySession_ReplacesRetrySetInPlace(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	session := OrchestraSession{
		ID:    "legacy-update",
		Panes: map[string]string{"claude": "%41", "codex": "%42"},
	}
	path := writeLegacySessionForTest(t, session)
	session.Panes = map[string]string{"codex": "%42"}

	require.NoError(t, UpdateSession(session))

	loaded, err := LoadSession(session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.Panes, loaded.Panes)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
