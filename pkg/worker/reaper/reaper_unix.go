//go:build linux || darwin || freebsd || openbsd || netbsd

package reaper

import (
	"syscall"
)

// unixDetector detects zombie processes using waitpid with WNOHANG.
// It scans by attempting to reap any child process, returning PIDs that were reaped.
type unixDetector struct{}

// DetectZombies attempts a non-blocking wait for any child process.
// Returns a slice of zombie PIDs that are ready to be reaped.
// Safe to call even if there are no child processes.
// @AX:WARN[AUTO]: syscall.Wait4 with pid=-1 reaps ANY child, including those spawned by other packages — verify no other waiters exist in the process
func (d *unixDetector) DetectZombies() []int {
	var pids []int
	for {
		var ws syscall.WaitStatus
		// Wait4 with pid=-1 waits for any child; WNOHANG returns immediately if none ready.
		pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
		if pid <= 0 || err != nil {
			break
		}
		pids = append(pids, pid)
	}
	return pids
}

// newDefaultDetector returns the Unix zombie detector.
func newDefaultDetector() ZombieDetector {
	return &unixDetector{}
}

// reapPID performs a targeted waitpid for a specific PID on Unix.
// It is safe to call even if the process no longer exists.
func reapPID(pid int) {
	var ws syscall.WaitStatus
	syscall.Wait4(pid, &ws, syscall.WNOHANG, nil) //nolint:errcheck
}
