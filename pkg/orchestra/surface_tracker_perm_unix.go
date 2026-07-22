//go:build !windows

package orchestra

import (
	"errors"
	"io/fs"
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

func trackerInfoOwnedByCurrentUser(info fs.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && uint32(os.Getuid()) == stat.Uid
}

func trackerModeSecure(info fs.FileInfo, want os.FileMode) bool {
	return info.Mode().Perm() == want
}

func trackerDirectorySyncUnavailable(err error) bool {
	return errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.ENOSYS)
}
