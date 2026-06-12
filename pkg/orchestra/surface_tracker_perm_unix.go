//go:build !windows

package orchestra

import (
	"os"
	"syscall"
)

// surfaceDirSecure reports whether the tracking directory is owned by the
// current user with no group/other permission bits (mode 0700). A mismatch
// indicates a privilege or symlink-swap attack on the shared base directory
// and causes surface tracking to be silently skipped (REQ-007).
func surfaceDirSecure(dir string) bool {
	var stat syscall.Stat_t
	if err := syscall.Stat(dir, &stat); err != nil {
		return false
	}
	return uint32(os.Getuid()) == stat.Uid && stat.Mode&0o077 == 0
}
