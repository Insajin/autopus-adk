//go:build linux

package promptlayer

import (
	"errors"

	"golang.org/x/sys/unix"
)

func currentOMPContextPromotionExecutableSHA256V3() (string, error) {
	// /proc/self/exe is a kernel-owned magic link to the live executable inode.
	// O_NOFOLLOW cannot be combined with opening this magic link for reading.
	fd, err := unix.Open("/proc/self/exe", unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return "", errors.New("open live OMP context promotion runtime executable")
	}
	var opened unix.Stat_t
	if unix.Fstat(fd, &opened) != nil {
		_ = unix.Close(fd)
		return "", errors.New("inspect live OMP context promotion runtime executable")
	}
	return hashOMPContextPromotionExecutableFDV3(fd, "self executable", opened)
}
