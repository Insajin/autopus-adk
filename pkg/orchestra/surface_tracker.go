package orchestra

import (
	"context"
	"errors"
	"io/fs"
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

// validSurfaceRef matches safe cmux and tmux surface reference formats.
var validSurfaceRef = regexp.MustCompile(`^([A-Za-z]+:[0-9]+|%[0-9]+)$`)

// surfaceTrackerRoot returns the preferred base directory for surface tracking
// files. It prefers ~/.autopus/surfaces and falls back to TempDir when the home
// directory is unavailable.
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

// splitTrackedPane owns tracking and cleanup for every non-empty SplitPane
// result, including a pane ID returned together with an error. On the first call
// it also reaps surfaces whose orchestrator processes are no longer alive.
func splitTrackedPane(ctx context.Context, term terminal.Terminal, dir terminal.Direction) (terminal.PaneID, error) {
	reapOrphanSurfacesOnce.Do(func() { ReapOrphanSurfaces(term) })
	paneID, err := term.SplitPane(ctx, dir)
	if paneID != "" {
		trackSurfaceForTerminal(term, string(paneID))
		if err != nil {
			closePaneSurface(term, paneID)
		}
	}
	return paneID, err
}

// trackSurface records a legacy surface ref using an atomic replacement. It is
// best-effort because tracking failures must never block pane creation.
func trackSurface(ref string) {
	trackSurfaceRecord(trackedSurface{Ref: ref})
}

func trackSurfaceForTerminal(term terminal.Terminal, ref string) {
	tracked, err := trackedSurfaceForTerminal(term, ref)
	if err != nil {
		if term == nil || term.Name() == "plain" {
			return
		}
		log.Printf("[surface-tracker] recording unresolved %s ref %q: %v", term.Name(), ref, err)
		tracked = trackedSurface{Ref: ref, TerminalKind: term.Name()}
	}
	trackSurfaceRecord(tracked)
}

func trackSurfaceRecord(tracked trackedSurface) {
	if tracked.Ref == "" {
		return
	}
	surfaceTrackerMu.Lock()
	defer surfaceTrackerMu.Unlock()
	root, err := openSecureTrackerRoot(surfaceTrackerBase, true)
	if err != nil {
		log.Printf("[surface-tracker] refusing tracker write: %v", err)
		return
	}
	defer func() { _ = root.Close() }()
	name := filepath.Base(surfaceTrackerFile(os.Getpid()))
	current, _, readErr := readTrackedSurfacesFromRoot(root, name)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		log.Printf("[surface-tracker] refusing tracker update: %v", readErr)
		return
	}
	current = append(current, tracked)
	if err := writeTrackedSurfacesToRoot(root, name, current, nil); err != nil {
		log.Printf("[surface-tracker] tracker write failed: %v", err)
	}
}

// untrackSurface removes a closed surface ref from this process's tracking file.
// A missed untrack is harmless: a stale ref is only ever re-closed (a no-op
// close of an already-gone surface) when this PID is later reaped.
func untrackSurface(ref string) {
	if ref == "" {
		return
	}
	if err := untrackSurfaceWithoutTerminal(ref); err != nil {
		log.Printf("[surface-tracker] untrack ref %q failed: %v", ref, err)
	}
}

// untrackSurfaceForTerminal removes only the exact backend/workspace/server
// tuple. It returns persistence failures so ownership handoff can fail closed.
func untrackSurfaceForTerminal(term terminal.Terminal, ref string) error {
	target, err := trackedSurfaceForTerminal(term, ref)
	if err != nil {
		return err
	}
	return mutateCurrentTracker(func(item trackedSurface) (bool, error) {
		return sameTrackedSurface(item, target), nil
	})
}

func untrackSurfaceWithoutTerminal(ref string) error {
	return mutateCurrentTracker(func(item trackedSurface) (bool, error) {
		return item.Ref == ref, nil
	})
}

func mutateCurrentTracker(match func(trackedSurface) (bool, error)) error {
	surfaceTrackerMu.Lock()
	defer surfaceTrackerMu.Unlock()
	root, err := openSecureTrackerRoot(surfaceTrackerBase, false)
	if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	name := filepath.Base(surfaceTrackerFile(os.Getpid()))
	tracked, _, err := readTrackedSurfacesFromRoot(root, name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	matched := make([]bool, len(tracked))
	identities := make(map[surfaceIdentity]struct{})
	legacyMatches := 0
	matchCount := 0
	for index, item := range tracked {
		matched[index], err = match(item)
		if err != nil {
			return err
		}
		if !matched[index] {
			continue
		}
		matchCount++
		identity, identityErr := trackedSurfaceIdentity(item)
		if identityErr != nil {
			legacyMatches++
			continue
		}
		identities[identity] = struct{}{}
	}
	if len(identities)+legacyMatches > 1 {
		return errors.New("surface ref is ambiguous across tracker identities")
	}
	if matchCount == 0 {
		return nil
	}
	kept := make([]trackedSurface, 0, len(tracked))
	for index, item := range tracked {
		if !matched[index] {
			kept = append(kept, item)
		}
	}
	return writeTrackedSurfacesToRoot(root, name, kept, nil)
}

// ReapOrphanSurfaces closes surfaces recorded by orchestrator processes that are
// no longer alive and removes their tracking files. It never touches the current
// process's surfaces or those of a live process, so it is safe to run while a
// concurrent orchestra holds its own panes. Structured cmux records can restore
// their persisted workspace; tmux records require the active server identity to
// match. Legacy cmux refs remain untouched because their workspace is unknown.
//
// Each ref is validated against validSurfaceRef before being passed to Close;
// refs that fail validation are logged and skipped (REQ-007). The legacy
// TempDir-based path is also inspected without creating it.
func ReapOrphanSurfaces(term terminal.Terminal) {
	if term == nil {
		return
	}
	reapOrphanSurfacesFromDir(surfaceTrackerBase, term)
	if surfaceTrackerLegacyBase != surfaceTrackerBase {
		reapOrphanSurfacesFromDir(surfaceTrackerLegacyBase, term)
	}
}

// reapOrphanSurfacesFromDir reaps compatible refs and retains retryable refs.
func reapOrphanSurfacesFromDir(dir string, term terminal.Terminal) {
	surfaceTrackerMu.Lock()
	defer surfaceTrackerMu.Unlock()
	root, err := openSecureTrackerRoot(dir, false)
	if err != nil {
		return
	}
	defer func() { _ = root.Close() }()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return
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
		// Refresh claims only after the owner is known to be dead. A yielding
		// process saves its durable session before it exits, so this ordering
		// closes the snapshot race between session persistence and process exit.
		claims := loadPersistedSessionSurfaceClaims()
		path := filepath.Join(dir, e.Name())
		tracked, _, readErr := readTrackedSurfacesFromRoot(root, e.Name())
		if readErr != nil {
			log.Printf("[surface-tracker] refusing entry %s: %v", path, readErr)
			continue
		}
		kept := tracked[:0]
		for _, item := range tracked {
			ref := item.Ref
			if !validSurfaceRef.MatchString(ref) {
				log.Printf("[surface-tracker] retaining invalid or malformed ref %q from %s", ref, path)
				kept = append(kept, item)
				continue
			}
			if claims.owns(item, term) {
				continue
			}
			closeTerminal, resolveErr := resolveTrackedSurfaceTerminal(term, item)
			if resolveErr != nil {
				log.Printf("[surface-tracker] retaining ref %q from %s: %v", ref, path, resolveErr)
				kept = append(kept, item)
				continue
			}
			if err := closeTerminal.Close(context.Background(), ref); err != nil {
				kept = append(kept, item)
			}
		}
		if err := writeTrackedSurfacesToRoot(root, e.Name(), kept, nil); err != nil {
			log.Printf("[surface-tracker] preserving old tracker %s after update failure: %v", path, err)
		}
	}
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
