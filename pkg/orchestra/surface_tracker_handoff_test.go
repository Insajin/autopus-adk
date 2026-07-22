package orchestra

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReapOrphanSurfaces_PersistedCmuxSessionOwnsTrackedPane(t *testing.T) {
	trackerDir := isolateTrackerAndSessionStorage(t)
	session := OrchestraSession{
		ID:           "handoff-protected-" + NewSessionID(),
		TerminalKind: "cmux",
		WorkspaceRef: "workspace:13",
		Panes:        map[string]string{"claude": "surface:41"},
		CreatedAt:    time.Now(),
	}
	require.NoError(t, SaveSession(session))
	t.Cleanup(func() { _ = RemoveSession(session.ID) })
	trackerFile := filepath.Join(trackerDir, "2147480130.surfaces")
	writeTrackedSurfaces(trackerFile, trackedCmuxSurfaces("workspace:13", "surface:41"))
	term := newTrackerContextTerminal("workspace:current")

	ReapOrphanSurfaces(term)

	assert.Empty(t, term.state.closeCalls, "a durable session owns the pane")
	_, err := os.Lstat(trackerFile)
	assert.ErrorIs(t, err, os.ErrNotExist, "handoff record should leave the orphan queue")
	_, err = LoadSession(session.ID)
	assert.NoError(t, err, "the durable cleanup handle remains authoritative")
}

func TestReapOrphanSurfaces_DifferentPersistedWorkspaceDoesNotClaimPane(t *testing.T) {
	trackerDir := isolateTrackerAndSessionStorage(t)
	session := OrchestraSession{
		ID:           "handoff-other-workspace-" + NewSessionID(),
		TerminalKind: "cmux",
		WorkspaceRef: "workspace:21",
		Panes:        map[string]string{"claude": "surface:41"},
		CreatedAt:    time.Now(),
	}
	require.NoError(t, SaveSession(session))
	t.Cleanup(func() { _ = RemoveSession(session.ID) })
	trackerFile := filepath.Join(trackerDir, "2147480131.surfaces")
	writeTrackedSurfaces(trackerFile, trackedCmuxSurfaces("workspace:13", "surface:41"))
	term := newTrackerContextTerminal("workspace:current")

	ReapOrphanSurfaces(term)

	assert.Equal(t, []string{"surface:41"}, term.state.closeCalls)
	assert.Equal(t, []string{"workspace:13"}, term.state.closeWorkspaces)
}

func TestReapOrphanSurfaces_PersistedTmuxSessionOwnsTrackedPane(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,12345,7")
	serverRef, ok := currentTmuxServerRef()
	require.True(t, ok)
	trackerDir := isolateTrackerAndSessionStorage(t)
	session := OrchestraSession{
		ID:            "handoff-tmux-" + NewSessionID(),
		TerminalKind:  "tmux",
		TmuxServerRef: serverRef,
		Panes:         map[string]string{"claude": "%41"},
		CreatedAt:     time.Now(),
	}
	require.NoError(t, SaveSession(session))
	t.Cleanup(func() { _ = RemoveSession(session.ID) })
	trackerFile := filepath.Join(trackerDir, "2147480133.surfaces")
	writeTrackedSurfaces(trackerFile, []trackedSurface{{
		Ref: "%41", TerminalKind: "tmux", TmuxServerRef: serverRef,
	}})
	term := &mockTerminal{name: "tmux"}

	ReapOrphanSurfaces(term)

	assert.Empty(t, term.closeCalls, "a durable session owns the pane")
	_, err := os.Lstat(trackerFile)
	assert.ErrorIs(t, err, os.ErrNotExist, "handoff record should leave the orphan queue")
}

func TestReapOrphanSurfaces_DifferentPersistedTmuxServerDoesNotClaimPane(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,12345,7")
	currentRef, ok := currentTmuxServerRef()
	require.True(t, ok)
	trackerDir := isolateTrackerAndSessionStorage(t)
	session := OrchestraSession{
		ID:            "handoff-other-tmux-" + NewSessionID(),
		TerminalKind:  "tmux",
		TmuxServerRef: "tmux-sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Panes:         map[string]string{"claude": "%41"},
		CreatedAt:     time.Now(),
	}
	require.NoError(t, SaveSession(session))
	t.Cleanup(func() { _ = RemoveSession(session.ID) })
	trackerFile := filepath.Join(trackerDir, "2147480134.surfaces")
	writeTrackedSurfaces(trackerFile, []trackedSurface{{
		Ref: "%41", TerminalKind: "tmux", TmuxServerRef: currentRef,
	}})
	term := &mockTerminal{name: "tmux"}

	ReapOrphanSurfaces(term)

	assert.Equal(t, []string{"%41"}, term.closeCalls)
}

func TestReapOrphanSurfaces_LegacyTmuxSessionOwnsLegacyTracker(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,12345,7")
	trackerDir := isolateTrackerAndSessionStorage(t)
	session := OrchestraSession{
		ID:        "legacy-handoff-" + NewSessionID(),
		Panes:     map[string]string{"claude": "%43"},
		CreatedAt: time.Now(),
	}
	name, err := legacySessionFilename(session.ID)
	require.NoError(t, err)
	data, err := json.Marshal(session)
	require.NoError(t, err)
	legacyPath := filepath.Join(os.TempDir(), name)
	require.NoError(t, os.WriteFile(legacyPath, data, 0o600))
	t.Cleanup(func() { _ = os.Remove(legacyPath) })
	trackerFile := filepath.Join(trackerDir, "2147480135.surfaces")
	writeTrackerRefs(trackerFile, []string{"%43"})
	term := &mockTerminal{name: "tmux"}

	ReapOrphanSurfaces(term)

	assert.Empty(t, term.closeCalls)
	_, err = os.Lstat(trackerFile)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestReapOrphanSurfaces_InsecureSessionFileDoesNotClaimPane(t *testing.T) {
	trackerDir := isolateTrackerAndSessionStorage(t)
	session := OrchestraSession{
		ID:           "handoff-insecure-session-" + NewSessionID(),
		TerminalKind: "cmux",
		WorkspaceRef: "workspace:13",
		Panes:        map[string]string{"claude": "surface:42"},
		CreatedAt:    time.Now(),
	}
	require.NoError(t, SaveSession(session))
	path := sessionFilePath(session.ID)
	require.NoError(t, os.Chmod(path, 0o644))
	t.Cleanup(func() {
		_ = os.Chmod(path, 0o600)
		_ = RemoveSession(session.ID)
	})
	trackerFile := filepath.Join(trackerDir, "2147480132.surfaces")
	writeTrackedSurfaces(trackerFile, trackedCmuxSurfaces("workspace:13", "surface:42"))
	term := newTrackerContextTerminal("workspace:current")

	ReapOrphanSurfaces(term)

	assert.Equal(t, []string{"surface:42"}, term.state.closeCalls)
}

func isolateTrackerAndSessionStorage(t *testing.T) string {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	originalBase := surfaceTrackerBase
	originalLegacy := surfaceTrackerLegacyBase
	trackerDir := secureTrackerTestDir(t)
	surfaceTrackerBase = trackerDir
	surfaceTrackerLegacyBase = filepath.Join(secureTrackerTestDir(t), "legacy-missing")
	t.Cleanup(func() {
		surfaceTrackerBase = originalBase
		surfaceTrackerLegacyBase = originalLegacy
	})
	return trackerDir
}
