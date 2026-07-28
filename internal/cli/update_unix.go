//go:build !windows

package cli

import (
	"syscall"
)

// isWritable checks if the directory is writable by the current user.
func isWritable(dir string) bool {
	return syscall.Access(dir, syscall.O_RDWR) == nil
}
