package orchestra

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
)

type trackerTerminalState struct {
	closeCalls      []string
	closeWorkspaces []string
	closeErr        error
}

type trackerContextTerminal struct {
	*mockTerminal
	workspaceRef string
	state        *trackerTerminalState
}

func newTrackerContextTerminal(workspaceRef string) *trackerContextTerminal {
	return &trackerContextTerminal{
		mockTerminal: &mockTerminal{name: "cmux"},
		workspaceRef: workspaceRef,
		state:        &trackerTerminalState{},
	}
}

func (t *trackerContextTerminal) WorkspaceRef() (string, error) {
	return t.workspaceRef, nil
}

func (t *trackerContextTerminal) WithWorkspaceRef(ref string) (terminal.Terminal, error) {
	return &trackerContextTerminal{
		mockTerminal: t.mockTerminal, workspaceRef: ref, state: t.state,
	}, nil
}

func (t *trackerContextTerminal) Close(_ context.Context, ref string) error {
	t.state.closeCalls = append(t.state.closeCalls, ref)
	t.state.closeWorkspaces = append(t.state.closeWorkspaces, t.workspaceRef)
	return t.state.closeErr
}

func trackedCmuxSurfaces(workspace string, refs ...string) []trackedSurface {
	tracked := make([]trackedSurface, 0, len(refs))
	for _, ref := range refs {
		tracked = append(tracked, trackedSurface{
			Ref: ref, TerminalKind: "cmux", WorkspaceRef: workspace,
		})
	}
	return tracked
}

func TestTrackSurfaceForTerminal_PersistsBackendAndWorkspace(t *testing.T) {
	orig := surfaceTrackerBase
	surfaceTrackerBase = filepath.Join(secureTrackerTestDir(t), "surfaces")
	defer func() { surfaceTrackerBase = orig }()
	term := newTrackerContextTerminal("workspace:13")

	trackSurfaceForTerminal(term, "surface:1414")

	assert.Equal(t, []trackedSurface{{
		Ref: "surface:1414", TerminalKind: "cmux", WorkspaceRef: "workspace:13",
	}}, readTrackedSurfaces(surfaceTrackerFile(os.Getpid())))
}

func TestReapOrphanSurfaces_LegacyTmuxGlobalPane_RequiresActiveTmux(t *testing.T) {
	orig := surfaceTrackerBase
	surfaceTrackerBase = secureTrackerTestDir(t)
	defer func() { surfaceTrackerBase = orig }()
	trackerFile := surfaceTrackerFile(2147480006)
	writeTrackerRefs(trackerFile, []string{"%42"})

	term := newTrackerContextTerminal("workspace:13")
	ReapOrphanSurfaces(term)

	assert.Empty(t, term.state.closeCalls)
	assert.Equal(t, []string{"%42"}, readTrackerRefs(trackerFile))
}

func TestReapOrphanSurfaces_LegacyCmuxRecordWithoutWorkspace_FailsClosed(t *testing.T) {
	orig := surfaceTrackerBase
	surfaceTrackerBase = secureTrackerTestDir(t)
	defer func() { surfaceTrackerBase = orig }()
	trackerFile := surfaceTrackerFile(2147480008)
	writeTrackerRefs(trackerFile, []string{"surface:88"})
	term := newTrackerContextTerminal("workspace:current")

	ReapOrphanSurfaces(term)

	assert.Empty(t, term.state.closeCalls)
	assert.Equal(t, []string{"surface:88"}, readTrackerRefs(trackerFile))
}

func TestReapOrphanSurfaces_PersistedCmuxFromTmuxContext_RestoresBackend(t *testing.T) {
	orig := surfaceTrackerBase
	surfaceTrackerBase = secureTrackerTestDir(t)
	originalFactory := newTrackedCmuxTerminal
	restored := newTrackerContextTerminal("workspace:13")
	newTrackedCmuxTerminal = func(string) (terminal.Terminal, error) { return restored, nil }
	defer func() {
		surfaceTrackerBase = orig
		newTrackedCmuxTerminal = originalFactory
	}()
	trackerFile := surfaceTrackerFile(2147480009)
	writeTrackedSurfaces(trackerFile, trackedCmuxSurfaces("workspace:13", "surface:99"))

	ReapOrphanSurfaces(&mockTerminal{name: "tmux"})

	assert.Equal(t, []string{"surface:99"}, restored.state.closeCalls)
	assert.Equal(t, []string{"workspace:13"}, restored.state.closeWorkspaces)
	_, err := os.Stat(trackerFile)
	assert.ErrorIs(t, err, os.ErrNotExist)
}
