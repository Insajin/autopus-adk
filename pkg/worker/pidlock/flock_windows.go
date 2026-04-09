//go:build windows

package pidlock

import (
	"fmt"
	"os/exec"
)

// flockFunc is a no-op on Windows; advisory locking is not supported.
// PID-file based mutual exclusion still works via file creation semantics.
var flockFunc = func(_ int, _ int) error { return nil }

// lock constants — unused on Windows but required for compilation.
const (
	lockEX = 0
	lockNB = 0
	lockUN = 0
)

// isProcessAlive returns true if a process with the given PID is running.
func isProcessAlive(pid int) bool {
	// On Windows, os.FindProcess always succeeds regardless of process state.
	// Use tasklist to check if the PID actually exists.
	err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").Run()
	return err == nil
}
