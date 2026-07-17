//go:build !windows

package cli

import "syscall"

func processAliveForTest(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
