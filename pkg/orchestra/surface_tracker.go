package orchestra

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// Surface orphan tracking (SPEC-ORCH-022 follow-up).
//
// cmux/tmux surfaces created for interactive provider panes are normally closed
// by cleanupInteractivePanes / WarmPool.cleanupPane via a defer. When the
// orchestrator process is killed (SIGKILL, crash) the defer never runs and the
// surface leaks. Each created surface ref is recorded under the owning
// orchestrator PID; a later run reaps surfaces whose owner PID is no longer
// alive. Surfaces owned by a live process (a concurrent orchestra, or this
// process itself) are NEVER touched, so reaping is safe under concurrency.

// surfaceTrackerBase is the directory holding per-PID surface tracking files.
// It prefers the user home directory and falls back to TempDir.
// It is a variable so tests can redirect it to an isolated temp directory.
var surfaceTrackerBase = surfaceTrackerRoot()

// surfaceTrackerLegacyBase is the old TempDir-based tracking path.
// ReapOrphanSurfaces reads from it without creating it (read-only reap).
// It is a variable so tests can redirect it.
var surfaceTrackerLegacyBase = filepath.Join(os.TempDir(), "autopus", "surfaces")

// validSurfaceRef matches the cmux ref format (e.g. "surface:3", "pane:7",
// "workspace:1"). Refs not matching this pattern are skipped by ReapOrphanSurfaces
// to prevent injection of shell metacharacters into terminal.Close.
var validSurfaceRef = regexp.MustCompile(`^[A-Za-z]+:[0-9]+$`)

// surfaceTrackerRoot returns the preferred base directory for surface tracking
// files. It prefers ~/.autopus/surfaces and falls back to TempDir when the home
// directory is unavailable. Callers (trackSurface) are responsible for creating
// the directory with MkdirAll and verifying ownership/mode before writing.
func surfaceTrackerRoot() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".autopus", "surfaces")
	}
	return filepath.Join(os.TempDir(), "autopus", "surfaces")
}

// reapOrphanSurfacesOnce ensures orphan reaping runs at most once per process,
// triggered lazily by the first interactive pane creation.
var reapOrphanSurfacesOnce sync.Once

func surfaceTrackerFile(pid int) string {
	return filepath.Join(surfaceTrackerBase, strconv.Itoa(pid)+".surfaces")
}

// splitTrackedPane splits a pane via the terminal and records the resulting
// surface ref for orphan reaping. On the first call in this process it also reaps
// surfaces left behind by orchestrator processes that are no longer alive.
func splitTrackedPane(ctx context.Context, term terminal.Terminal, dir terminal.Direction) (terminal.PaneID, error) {
	reapOrphanSurfacesOnce.Do(func() { ReapOrphanSurfaces(term) })
	paneID, err := term.SplitPane(ctx, dir)
	if err == nil && paneID != "" {
		trackSurface(string(paneID))
	}
	return paneID, err
}

// trackSurface appends a surface ref to this process's tracking file (best-effort;
// tracking failures must never block pane creation). Before writing, it verifies
// that the tracking directory is owned by the current user with mode 0700 (no
// group/other bits). A mismatch indicates a privilege or symlink-swap attack and
// causes the write to be silently skipped (REQ-007).
func trackSurface(ref string) {
	if ref == "" {
		return
	}
	if err := os.MkdirAll(surfaceTrackerBase, 0o700); err != nil {
		return
	}
	// Security: verify ownership and mode before writing (platform-specific).
	if !surfaceDirSecure(surfaceTrackerBase) {
		return
	}
	f, err := os.OpenFile(surfaceTrackerFile(os.Getpid()), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.WriteString(ref + "\n")
}

// untrackSurface removes a closed surface ref from this process's tracking file.
// A missed untrack is harmless: a stale ref is only ever re-closed (a no-op
// close of an already-gone surface) when this PID is later reaped.
func untrackSurface(ref string) {
	if ref == "" {
		return
	}
	path := surfaceTrackerFile(os.Getpid())
	refs := readTrackerRefs(path)
	if len(refs) == 0 {
		return
	}
	kept := refs[:0]
	for _, r := range refs {
		if r != ref {
			kept = append(kept, r)
		}
	}
	writeTrackerRefs(path, kept)
}

// ReapOrphanSurfaces closes surfaces recorded by orchestrator processes that are
// no longer alive and removes their tracking files. It never touches the current
// process's surfaces or those of a live process, so it is safe to run while a
// concurrent orchestra holds its own panes. It is a no-op for terminals that
// cannot host surfaces (plain) so cmux tracking files are not discarded without
// actually closing their surfaces.
//
// Each ref is validated against validSurfaceRef before being passed to Close;
// refs that fail validation are logged and skipped (REQ-007). The legacy
// TempDir-based path is also reaped read-only (no MkdirAll).
func ReapOrphanSurfaces(term terminal.Terminal) {
	if term == nil || term.Name() == "plain" {
		return
	}
	reapOrphanSurfacesFromDir(surfaceTrackerBase, term)
	if surfaceTrackerLegacyBase != surfaceTrackerBase {
		reapOrphanSurfacesFromDir(surfaceTrackerLegacyBase, term)
	}
}

// reapOrphanSurfacesFromDir performs orphan reaping from a single tracking
// directory without creating it. Refs are validated before Close is called.
func reapOrphanSurfacesFromDir(dir string, term terminal.Terminal) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // dir absent or unreadable — silent no-op
	}
	self := os.Getpid()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".surfaces") {
			continue
		}
		pid, perr := strconv.Atoi(strings.TrimSuffix(e.Name(), ".surfaces"))
		if perr != nil || pid == self || processAlive(pid) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		for _, ref := range readTrackerRefs(path) {
			if !validSurfaceRef.MatchString(ref) {
				log.Printf("[surface-tracker] skipping invalid ref %q from %s", ref, path)
				continue
			}
			_ = term.Close(context.Background(), ref)
		}
		_ = os.Remove(path)
	}
}

// readTrackerRefs returns the non-empty surface refs recorded in a tracking file.
func readTrackerRefs(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var refs []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			refs = append(refs, line)
		}
	}
	return refs
}

// writeTrackerRefs rewrites a tracking file, removing it entirely when empty.
func writeTrackerRefs(path string, refs []string) {
	if len(refs) == 0 {
		_ = os.Remove(path)
		return
	}
	_ = os.WriteFile(path, []byte(strings.Join(refs, "\n")+"\n"), 0o600)
}

// processAlive reports whether a process with the given PID currently exists.
// Signal 0 performs existence/permission checking without delivering a signal.
// On PID reuse a dead orchestrator can appear alive; the only consequence is a
// deferred reap (the surface lingers one more cycle), never a wrongful close of a
// live process's surface.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
