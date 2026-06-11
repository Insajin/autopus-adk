package orchestra

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain isolates surface tracker writes to a throwaway directory so neither
// the real temp tree nor concurrent test runs are polluted by tracking files,
// and so the lazy reap triggered by splitTrackedPane in other tests sees an
// empty (no-op) directory.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "autopus-surface-tracker-test-")
	if err == nil {
		surfaceTrackerBase = filepath.Join(tmp, "surfaces")
	}
	code := m.Run()
	if tmp != "" {
		_ = os.RemoveAll(tmp)
	}
	os.Exit(code)
}

func TestProcessAlive(t *testing.T) {
	t.Parallel()

	assert.True(t, processAlive(os.Getpid()), "current process must be reported alive")
	assert.False(t, processAlive(-1), "negative pid is never alive")
	assert.False(t, processAlive(0), "pid 0 is never alive")
	// A very large pid is overwhelmingly unlikely to map to a live process.
	assert.False(t, processAlive(2147480000), "unused high pid must be reported dead")
}

func TestTrackAndUntrackSurface(t *testing.T) {
	orig := surfaceTrackerBase
	surfaceTrackerBase = t.TempDir()
	defer func() { surfaceTrackerBase = orig }()

	trackSurface("surface:1")
	trackSurface("surface:2")
	require.ElementsMatch(t, []string{"surface:1", "surface:2"}, readTrackerRefs(surfaceTrackerFile(os.Getpid())))

	untrackSurface("surface:1")
	assert.Equal(t, []string{"surface:2"}, readTrackerRefs(surfaceTrackerFile(os.Getpid())))

	// Removing the last ref deletes the file entirely.
	untrackSurface("surface:2")
	_, err := os.Stat(surfaceTrackerFile(os.Getpid()))
	assert.True(t, os.IsNotExist(err), "empty tracker file must be removed")

	// Untracking an absent ref or empty file is a no-op (no panic).
	untrackSurface("surface:absent")
	trackSurface("") // ignored, must not create a file
}

func TestReapOrphanSurfaces_ClosesOnlyDeadOwners(t *testing.T) {
	orig := surfaceTrackerBase
	surfaceTrackerBase = t.TempDir()
	defer func() { surfaceTrackerBase = orig }()

	deadPID := 2147480001 // unused high pid → reported dead
	writeTrackerRefs(surfaceTrackerFile(deadPID), []string{"surface:10", "surface:11"})
	// A live owner that must NOT be reaped: this process.
	selfRefs := []string{"surface:20"}
	writeTrackerRefs(surfaceTrackerFile(os.Getpid()), selfRefs)

	term := &mockTerminal{name: "cmux"}
	ReapOrphanSurfaces(term)

	closed := append([]string(nil), term.closeCalls...)
	sort.Strings(closed)
	assert.Equal(t, []string{"surface:10", "surface:11"}, closed, "only dead owner's surfaces are closed")

	_, err := os.Stat(surfaceTrackerFile(deadPID))
	assert.True(t, os.IsNotExist(err), "dead owner's tracking file must be removed")
	assert.Equal(t, selfRefs, readTrackerRefs(surfaceTrackerFile(os.Getpid())), "self tracking file must be preserved")
}

func TestReapOrphanSurfaces_NoOpForPlainOrNilTerm(t *testing.T) {
	orig := surfaceTrackerBase
	surfaceTrackerBase = t.TempDir()
	defer func() { surfaceTrackerBase = orig }()

	deadPID := 2147480002
	writeTrackerRefs(surfaceTrackerFile(deadPID), []string{"surface:30"})

	// Plain terminal cannot close surfaces — the tracking file must be left intact
	// so a later cmux-capable run can still reap it.
	ReapOrphanSurfaces(&mockTerminal{name: "plain"})
	assert.Equal(t, []string{"surface:30"}, readTrackerRefs(surfaceTrackerFile(deadPID)))

	ReapOrphanSurfaces(nil)
	assert.Equal(t, []string{"surface:30"}, readTrackerRefs(surfaceTrackerFile(deadPID)))
}

func TestReapOrphanSurfaces_SkipsLivePeerOwner(t *testing.T) {
	orig := surfaceTrackerBase
	surfaceTrackerBase = t.TempDir()
	defer func() { surfaceTrackerBase = orig }()

	// Simulate a concurrent, still-running orchestrator with a real live child
	// process and use its PID as the tracking-file owner.
	cmd := exec.Command("sleep", "30")
	require.NoError(t, cmd.Start())
	peerPID := cmd.Process.Pid
	defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()

	writeTrackerRefs(surfaceTrackerFile(peerPID), []string{"surface:40"})

	term := &mockTerminal{name: "cmux"}
	ReapOrphanSurfaces(term)

	assert.Empty(t, term.closeCalls, "a live concurrent owner's surfaces must not be closed")
	assert.Equal(t, []string{"surface:40"}, readTrackerRefs(surfaceTrackerFile(peerPID)), "live owner's tracking file must be preserved")
}
