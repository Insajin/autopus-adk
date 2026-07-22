package orchestra

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentTmuxServerRef_UsesSocketAndServerPID(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,12345,7")
	first, ok := currentTmuxServerRef()
	require.True(t, ok)
	assert.Regexp(t, `^tmux-sha256:[0-9a-f]{64}$`, first)

	t.Setenv("TMUX", "/tmp/tmux-501/default,12345,99")
	sameServer, ok := currentTmuxServerRef()
	require.True(t, ok)
	assert.Equal(t, first, sameServer, "session index is not part of server identity")

	t.Setenv("TMUX", "/tmp/tmux-501/default,54321,7")
	differentPID, ok := currentTmuxServerRef()
	require.True(t, ok)
	assert.NotEqual(t, first, differentPID)

	t.Setenv("TMUX", "/tmp/tmux-501/other,12345,7")
	differentSocket, ok := currentTmuxServerRef()
	require.True(t, ok)
	assert.NotEqual(t, first, differentSocket)
}

func TestCurrentTmuxServerRef_InvalidEnvironmentFailsClosed(t *testing.T) {
	for _, value := range []string{
		"", "missing-fields", "/tmp/socket,abc,1", "relative,123,1", "/tmp/socket,123,not-a-session",
	} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("TMUX", value)
			ref, ok := currentTmuxServerRef()
			assert.False(t, ok)
			assert.Empty(t, ref)
		})
	}
}

func TestTrackedSurfaceIdentity_RejectsConflictingBackendContext(t *testing.T) {
	serverRef := "tmux-sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tests := []trackedSurface{
		{Ref: "surface:7", TerminalKind: "cmux", WorkspaceRef: "workspace:13", TmuxServerRef: serverRef},
		{Ref: "%7", TerminalKind: "tmux", WorkspaceRef: "workspace:13", TmuxServerRef: serverRef},
	}
	for _, tracked := range tests {
		_, err := trackedSurfaceIdentity(tracked)
		assert.Error(t, err)
	}
}

func TestUntrackSurfaceForTerminal_RemovesOnlyExactCmuxTuple(t *testing.T) {
	original := surfaceTrackerBase
	surfaceTrackerBase = secureTrackerTestDir(t)
	t.Cleanup(func() { surfaceTrackerBase = original })
	path := surfaceTrackerFile(os.Getpid())
	writeTrackedSurfaces(path, []trackedSurface{
		{Ref: "surface:7", TerminalKind: "cmux", WorkspaceRef: "workspace:13"},
		{Ref: "surface:7", TerminalKind: "cmux", WorkspaceRef: "workspace:21"},
	})
	term := newTrackerContextTerminal("workspace:13")

	require.NoError(t, untrackSurfaceForTerminal(term, "surface:7"))

	assert.Equal(t, []trackedSurface{{
		Ref: "surface:7", TerminalKind: "cmux", WorkspaceRef: "workspace:21",
	}}, readTrackedSurfaces(path))
}

func TestUntrackSurfaceForTerminal_RemovesOnlyExactTmuxServerTuple(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,12345,7")
	current, ok := currentTmuxServerRef()
	require.True(t, ok)
	other := "tmux-sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	original := surfaceTrackerBase
	surfaceTrackerBase = secureTrackerTestDir(t)
	t.Cleanup(func() { surfaceTrackerBase = original })
	path := surfaceTrackerFile(os.Getpid())
	writeTrackedSurfaces(path, []trackedSurface{
		{Ref: "%7", TerminalKind: "tmux", TmuxServerRef: current},
		{Ref: "%7", TerminalKind: "tmux", TmuxServerRef: other},
	})

	require.NoError(t, untrackSurfaceForTerminal(&mockTerminal{name: "tmux"}, "%7"))

	assert.Equal(t, []trackedSurface{{
		Ref: "%7", TerminalKind: "tmux", TmuxServerRef: other,
	}}, readTrackedSurfaces(path))
}

func TestUntrackSurface_AmbiguousStructuredRefIsPreserved(t *testing.T) {
	original := surfaceTrackerBase
	surfaceTrackerBase = secureTrackerTestDir(t)
	t.Cleanup(func() { surfaceTrackerBase = original })
	path := surfaceTrackerFile(os.Getpid())
	want := []trackedSurface{
		{Ref: "surface:7", TerminalKind: "cmux", WorkspaceRef: "workspace:13"},
		{Ref: "surface:7", TerminalKind: "cmux", WorkspaceRef: "workspace:21"},
	}
	writeTrackedSurfaces(path, want)

	untrackSurface("surface:7")

	assert.Equal(t, want, readTrackedSurfaces(path))
}

func TestReapOrphanSurfaces_TmuxServerMismatchRetainsHandle(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/origin,12345,1")
	origin, ok := currentTmuxServerRef()
	require.True(t, ok)
	t.Setenv("TMUX", "/tmp/tmux-501/current,12345,1")
	original := surfaceTrackerBase
	surfaceTrackerBase = secureTrackerTestDir(t)
	t.Cleanup(func() { surfaceTrackerBase = original })
	path := surfaceTrackerFile(2147480120)
	writeTrackedSurfaces(path, []trackedSurface{{
		Ref: "%42", TerminalKind: "tmux", TmuxServerRef: origin,
	}})
	term := &mockTerminal{name: "tmux"}

	ReapOrphanSurfaces(term)

	assert.Empty(t, term.closeCalls)
	assert.Equal(t, []string{"%42"}, readTrackerRefs(path))
}

func TestReapOrphanSurfaces_TmuxSameServerClosesHandle(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,12345,1")
	serverRef, ok := currentTmuxServerRef()
	require.True(t, ok)
	original := surfaceTrackerBase
	surfaceTrackerBase = secureTrackerTestDir(t)
	t.Cleanup(func() { surfaceTrackerBase = original })
	path := surfaceTrackerFile(2147480121)
	writeTrackedSurfaces(path, []trackedSurface{{
		Ref: "%42", TerminalKind: "tmux", TmuxServerRef: serverRef,
	}})
	term := &mockTerminal{name: "tmux"}

	ReapOrphanSurfaces(term)

	assert.Equal(t, []string{"%42"}, term.closeCalls)
	_, err := os.Lstat(path)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestReapOrphanSurfaces_TmuxHandleRequiresActiveTmuxContext(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,12345,1")
	serverRef, ok := currentTmuxServerRef()
	require.True(t, ok)
	original := surfaceTrackerBase
	surfaceTrackerBase = secureTrackerTestDir(t)
	t.Cleanup(func() { surfaceTrackerBase = original })
	path := surfaceTrackerFile(2147480122)
	writeTrackedSurfaces(path, []trackedSurface{{
		Ref: "%42", TerminalKind: "tmux", TmuxServerRef: serverRef,
	}})
	term := newTrackerContextTerminal("workspace:current")

	ReapOrphanSurfaces(term)

	assert.Empty(t, term.state.closeCalls)
	assert.Equal(t, []string{"%42"}, readTrackerRefs(path))
}

func TestTrackSurfaceForTerminal_TmuxPersistsServerIdentity(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,12345,1")
	original := surfaceTrackerBase
	surfaceTrackerBase = filepath.Join(secureTrackerTestDir(t), "surfaces")
	t.Cleanup(func() { surfaceTrackerBase = original })

	trackSurfaceForTerminal(&tmuxTrackerTerminal{}, "%9")

	tracked := readTrackedSurfaces(surfaceTrackerFile(os.Getpid()))
	require.Len(t, tracked, 1)
	assert.Equal(t, "tmux", tracked[0].TerminalKind)
	assert.NotEmpty(t, tracked[0].TmuxServerRef)
}

type tmuxTrackerTerminal struct{ mockTerminal }

func (*tmuxTrackerTerminal) Name() string                        { return "tmux" }
func (*tmuxTrackerTerminal) Close(context.Context, string) error { return nil }

var _ terminal.Terminal = (*tmuxTrackerTerminal)(nil)
