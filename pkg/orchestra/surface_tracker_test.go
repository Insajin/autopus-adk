package orchestra

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain isolates surface tracker writes to a throwaway directory so neither
// the real temp tree nor concurrent test runs are polluted by tracking files,
// and so the lazy reap triggered by splitTrackedPane in other tests sees an
// empty (no-op) directory.
//
// It also redirects HOME (and the macOS-specific home env vars) to a throwaway
// directory so reliabilityRuntimeRoot() — which derives ~/.autopus/runtime/
// orchestra from os.UserHomeDir() — writes failure bundles into the temp tree
// instead of polluting the developer's real ~/.autopus/runtime/orchestra/runs.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "autopus-surface-tracker-test-")
	if err == nil {
		surfaceTrackerBase = filepath.Join(tmp, "surfaces")
		// Isolate the home directory so os.UserHomeDir()-based runtime roots
		// resolve under the throwaway tree. HOME covers Unix/macOS; the rest
		// guard alternate resolution paths.
		fakeHome := filepath.Join(tmp, "home")
		if mkErr := os.MkdirAll(fakeHome, 0o700); mkErr == nil {
			for _, key := range []string{"HOME", "XDG_CACHE_HOME", "XDG_CONFIG_HOME"} {
				_ = os.Setenv(key, fakeHome)
			}
		}
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
	// Use a non-existing subdirectory so MkdirAll creates it with mode 0700,
	// satisfying the ownership/mode security check in trackSurface (REQ-007).
	surfaceTrackerBase = filepath.Join(t.TempDir(), "surfaces")
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

// TestSurfaceTrackerRoot_ReturnsHomePath verifies S9: surfaceTrackerRoot() returns
// a path under the user home directory when UserHomeDir is available, not under
// os.TempDir().
func TestSurfaceTrackerRoot_ReturnsHomePath(t *testing.T) {
	// This test verifies surfaceTrackerRoot()'s home-vs-TempDir preference, so it
	// must run against a home directory that is NOT under os.TempDir(). The
	// process-wide TestMain isolation points HOME at an os.MkdirTemp dir (under
	// TempDir on macOS), which would defeat the "not under TempDir" assertion.
	// Inject an explicit synthetic home outside TempDir for this test only;
	// surfaceTrackerRoot performs no filesystem writes, only path joins, so the
	// path need not exist.
	syntheticHome := filepath.Join(string(os.PathSeparator), "synthetic-home", "autopus-test-user")
	require.False(t, strings.HasPrefix(syntheticHome, os.TempDir()),
		"test precondition: synthetic home must not be under TempDir")
	t.Setenv("HOME", syntheticHome)

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("UserHomeDir unavailable in this environment")
	}
	root := surfaceTrackerRoot()
	require.True(t, strings.HasPrefix(root, home),
		"surfaceTrackerRoot must be under home dir; got %s", root)
	tmpDir := os.TempDir()
	assert.False(t, strings.HasPrefix(root, tmpDir),
		"surfaceTrackerRoot must not be under TempDir when home is available; got %s", root)
}

// TestTrackSurface_SkipsWhenDirModeNotSecure verifies S9: trackSurface does not
// write when the tracking directory mode has group/other permission bits set.
func TestTrackSurface_SkipsWhenDirModeNotSecure(t *testing.T) {
	orig := surfaceTrackerBase
	defer func() { surfaceTrackerBase = orig }()

	// Create a directory with mode 0755 (group/other read permitted).
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o755))
	surfaceTrackerBase = dir

	trackSurface("surface:99")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries,
		"tracking file must not be written when dir mode has group/other bits set")
}

// TestReapOrphanSurfaces_RefValidationAndLegacyNoCreate verifies S10:
// only refs matching the valid format are passed to Close; invalid refs are
// logged and skipped; the legacy /tmp path is not created by ReapOrphanSurfaces.
func TestReapOrphanSurfaces_RefValidationAndLegacyNoCreate(t *testing.T) {
	orig := surfaceTrackerBase
	origLegacy := surfaceTrackerLegacyBase
	defer func() {
		surfaceTrackerBase = orig
		surfaceTrackerLegacyBase = origLegacy
	}()

	base := t.TempDir()
	surfaceTrackerBase = base

	// Point legacy base to a path that does not exist; verify it is NOT created.
	legacyBase := filepath.Join(t.TempDir(), "legacy-never-created")
	surfaceTrackerLegacyBase = legacyBase

	// Write refs including two invalid ones and one valid ref for a dead PID.
	deadPID := 2147480004
	writeTrackerRefs(surfaceTrackerFile(deadPID),
		[]string{"--help", "; rm -rf /", "surface:3"})
	// Self entry must not be reaped.
	writeTrackerRefs(surfaceTrackerFile(os.Getpid()), []string{"surface:99"})

	term := &mockTerminal{name: "cmux"}
	logOutput := captureLog(t, func() {
		ReapOrphanSurfaces(term)
	})

	// Only the valid ref "surface:3" must be closed.
	assert.Equal(t, []string{"surface:3"}, term.closeCalls,
		"Close must receive exactly {surface:3}")

	// Invalid refs must appear in log output.
	assert.Contains(t, logOutput, "--help",
		"invalid ref --help must be logged")
	assert.Contains(t, logOutput, "; rm -rf /",
		"invalid ref ; rm -rf / must be logged")

	// Legacy path must not have been created.
	_, statErr := os.Stat(legacyBase)
	assert.True(t, os.IsNotExist(statErr),
		"legacy base must not be created by ReapOrphanSurfaces")
}

func TestReapOrphanSurfaces_TmuxClosesGlobalPaneRef(t *testing.T) {
	orig := surfaceTrackerBase
	surfaceTrackerBase = t.TempDir()
	defer func() { surfaceTrackerBase = orig }()

	deadPID := 2147480005
	writeTrackerRefs(surfaceTrackerFile(deadPID), []string{"%42"})

	term := &mockTerminal{name: "tmux"}
	ReapOrphanSurfaces(term)

	assert.Equal(t, []string{"%42"}, term.closeCalls,
		"tmux Close must receive exactly the valid global pane ref")
}

func TestReapOrphanSurfaces_CmuxPreservesTmuxGlobalPaneRef(t *testing.T) {
	orig := surfaceTrackerBase
	surfaceTrackerBase = t.TempDir()
	defer func() { surfaceTrackerBase = orig }()

	deadPID := 2147480006
	trackerFile := surfaceTrackerFile(deadPID)
	writeTrackerRefs(trackerFile, []string{"%42"})

	term := &mockTerminal{name: "cmux"}
	ReapOrphanSurfaces(term)

	assert.Empty(t, term.closeCalls, "cmux must not close a tmux global pane ref")
	assert.Equal(t, []string{"%42"}, readTrackerRefs(trackerFile),
		"incompatible ref must remain tracked for a later tmux reaper")
}

func TestReapOrphanSurfaces_CloseErrorPreservesRefForRetry(t *testing.T) {
	orig := surfaceTrackerBase
	surfaceTrackerBase = t.TempDir()
	defer func() { surfaceTrackerBase = orig }()

	deadPID := 2147480007
	trackerFile := surfaceTrackerFile(deadPID)
	writeTrackerRefs(trackerFile, []string{"surface:77"})

	term := &mockTerminal{name: "cmux", closeErr: errors.New("close failed")}
	ReapOrphanSurfaces(term)

	assert.Equal(t, []string{"surface:77"}, term.closeCalls,
		"compatible ref must be passed to Close")
	assert.Equal(t, []string{"surface:77"}, readTrackerRefs(trackerFile),
		"failed Close ref must remain tracked for retry")
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
