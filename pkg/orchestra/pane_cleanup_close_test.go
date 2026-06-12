package orchestra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
)

// isolateSurfaceTracker redirects the package-level surface tracker base to a
// per-test temp dir so closePaneSurface's untrack/track side effects never touch
// the real ~/.autopus/surfaces store. Tests using it must not run in parallel
// because surfaceTrackerBase is a shared global.
func isolateSurfaceTracker(t *testing.T) {
	t.Helper()
	orig := surfaceTrackerBase
	surfaceTrackerBase = filepath.Join(t.TempDir(), "surfaces")
	t.Cleanup(func() { surfaceTrackerBase = orig })
}

// flakyCloseTerminal fails Close for the first failUntil calls, then succeeds.
// It records every Close attempt so tests can assert the retry count.
type flakyCloseTerminal struct {
	mockTerminal
	failUntil    int // number of leading Close calls that return an error
	closeAttempt int // total Close attempts observed
}

func (m *flakyCloseTerminal) Close(_ context.Context, name string) error {
	m.closeAttempt++
	m.closeCalls = append(m.closeCalls, name)
	if m.closeAttempt <= m.failUntil {
		return fmt.Errorf("cmux: close surface %s: transient failure", name)
	}
	return nil
}

// TestClosePaneSurface_RetriesTransientFailure reproduces issue #61: a completed
// provider pane lingers when close-surface fails transiently at the watchdog
// boundary. The fix retries the close, so a single transient failure still ends
// in a closed surface.
func TestClosePaneSurface_RetriesTransientFailure(t *testing.T) {
	isolateSurfaceTracker(t)

	term := &flakyCloseTerminal{failUntil: 1}
	ok := closePaneSurface(term, terminal.PaneID("surface:7"))

	assert.True(t, ok, "surface must be closed after a transient failure is retried")
	assert.Equal(t, 2, term.closeAttempt, "close must be retried after the first transient failure")
}

// TestClosePaneSurface_SucceedsFirstTry guards the common path: exactly one Close
// call and no spurious retries when the surface closes cleanly.
func TestClosePaneSurface_SucceedsFirstTry(t *testing.T) {
	isolateSurfaceTracker(t)

	term := &flakyCloseTerminal{failUntil: 0}
	ok := closePaneSurface(term, terminal.PaneID("surface:3"))

	assert.True(t, ok)
	assert.Equal(t, 1, term.closeAttempt, "a clean close must run exactly once (idempotent, no retry)")
}

// TestClosePaneSurface_PersistentFailureKeepsRefTracked verifies that when close
// never succeeds the surface ref is LEFT in the tracking file so a later-run
// orphan reaper can still reclaim it. The previous code untracked the ref even on
// failure, turning the transient leak into a permanent one.
func TestClosePaneSurface_PersistentFailureKeepsRefTracked(t *testing.T) {
	isolateSurfaceTracker(t)

	ref := "surface:9"
	trackSurface(ref)

	term := &flakyCloseTerminal{failUntil: closePaneSurfaceAttempts + 1}
	ok := closePaneSurface(term, terminal.PaneID(ref))

	assert.False(t, ok, "a surface that never closes must report failure")
	assert.Equal(t, closePaneSurfaceAttempts, term.closeAttempt, "close must stop after the bounded retry budget")

	refs := readTrackerRefs(surfaceTrackerFile(os.Getpid()))
	assert.Contains(t, refs, ref, "a ref that failed to close must stay tracked for orphan reaping")
}

// TestClosePaneSurface_SuccessUntracksRef confirms the happy path untracks the
// ref so it is not re-closed (a no-op double close) during a later orphan reap.
func TestClosePaneSurface_SuccessUntracksRef(t *testing.T) {
	isolateSurfaceTracker(t)

	ref := "surface:11"
	trackSurface(ref)

	term := &flakyCloseTerminal{failUntil: 0}
	assert.True(t, closePaneSurface(term, terminal.PaneID(ref)))

	refs := readTrackerRefs(surfaceTrackerFile(os.Getpid()))
	assert.NotContains(t, refs, ref, "a successfully closed surface must be untracked")
}

// TestClosePaneSurface_EmptyRefNoop guards the idempotency edge: an empty pane id
// performs no close call.
func TestClosePaneSurface_EmptyRefNoop(t *testing.T) {
	t.Parallel()
	term := &flakyCloseTerminal{}
	assert.True(t, closePaneSurface(term, terminal.PaneID("")))
	assert.Zero(t, term.closeAttempt, "an empty ref must not trigger a close")
}

// TestCleanupInteractivePanes_ClosesEverySurface is the integration-level guard:
// cleanup must attempt a close for each pane it owns, regardless of a transient
// first failure (the leak class behind issue #61).
func TestCleanupInteractivePanes_ClosesEverySurface(t *testing.T) {
	isolateSurfaceTracker(t)

	term := &flakyCloseTerminal{failUntil: 1}
	panes := []paneInfo{
		{paneID: "surface:1"},
		{paneID: "surface:2"},
	}
	cleanupInteractivePanes(term, panes)

	assert.Contains(t, term.closeCalls, "surface:1")
	assert.Contains(t, term.closeCalls, "surface:2")
}
