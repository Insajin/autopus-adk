// Package pidlock provides advisory PID-based lock file management for single-instance enforcement.
package pidlock

// @AX:ANCHOR[AUTO]: public API contract — Acquire/Release form the single-instance enforcement boundary; callers depend on error semantics

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// flockFunc is the advisory lock function used by Acquire and Release.
// It can be overridden in tests to simulate flock failures.
var flockFunc = syscall.Flock

// Lock represents a PID lock file handle.
type Lock struct {
	path string
	file *os.File
}

// DefaultPath returns the default PID lock file path: ~/.autopus/worker.pid.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".autopus", "worker.pid")
	}
	return filepath.Join(home, ".autopus", "worker.pid")
}

// New creates a new Lock for the given path. No file I/O occurs at this point.
func New(path string) *Lock {
	return &Lock{path: path}
}

// Acquire attempts to acquire the PID lock file.
//
// If no lock file exists, it creates one containing the current PID.
// If the lock file exists but holds a dead process PID (stale), it logs a warning and reclaims.
// If the lock file exists and the process is alive, it returns an error.
func (l *Lock) Acquire() error {
	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return fmt.Errorf("pidlock: mkdir %s: %w", filepath.Dir(l.path), err)
	}

	// Check for existing lock file
	if _, err := os.Stat(l.path); err == nil {
		existing, readErr := readPIDFromFile(l.path)
		if readErr == nil && existing > 0 {
			if isProcessAlive(existing) {
				return fmt.Errorf("Worker already running (PID: %d)", existing)
			}
			// Stale lock: process is dead
			log.Printf("[pidlock] stale lock detected (PID %d no longer exists), reclaiming %s", existing, l.path)
		}
		// Remove stale or unreadable lock file before recreating
		_ = os.Remove(l.path)
	}

	// @AX:NOTE[AUTO]: magic constant — 0o600 restricts PID file to owner-only; changing breaks multi-user scenarios
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("pidlock: open %s: %w", l.path, err)
	}

	// Apply advisory flock (non-blocking)
	// @AX:WARN[AUTO]: syscall.Flock is Unix-only advisory lock — not portable; Windows builds will fail at compile time
	if err := flockFunc(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		_ = os.Remove(l.path)
		return fmt.Errorf("pidlock: flock %s: %w", l.path, err)
	}

	pid := os.Getpid()
	if _, err := fmt.Fprintf(f, "%d", pid); err != nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
		_ = os.Remove(l.path)
		return fmt.Errorf("pidlock: write PID: %w", err)
	}

	l.file = f
	return nil
}

// Release releases the PID lock by closing the file handle and deleting the lock file.
func (l *Lock) Release() error {
	if l.file != nil {
		_ = flockFunc(int(l.file.Fd()), syscall.LOCK_UN)
		_ = l.file.Close()
		l.file = nil
	}
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("pidlock: remove %s: %w", l.path, err)
	}
	return nil
}

// ReadPID reads the PID stored in the lock file.
func (l *Lock) ReadPID() (int, error) {
	return readPIDFromFile(l.path)
}

// readPIDFromFile reads an integer PID from the given file path.
func readPIDFromFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("pidlock: read %s: %w", path, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("pidlock: parse PID from %s: %w", path, err)
	}
	return pid, nil
}

// isProcessAlive returns true if a process with the given PID is running.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks process existence without sending an actual signal
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
