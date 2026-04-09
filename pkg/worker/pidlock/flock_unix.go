//go:build !windows

package pidlock

import (
	"os"
	"syscall"
)

// flockFunc is the advisory lock function used by Acquire and Release.
// It can be overridden in tests to simulate flock failures.
var flockFunc = syscall.Flock

// lock constants for advisory file locking.
const (
	lockEX = syscall.LOCK_EX
	lockNB = syscall.LOCK_NB
	lockUN = syscall.LOCK_UN
)

// isProcessAlive returns true if a process with the given PID is running.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks process existence without sending an actual signal.
	return proc.Signal(syscall.Signal(0)) == nil
}
