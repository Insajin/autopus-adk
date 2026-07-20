//go:build darwin

package run

import (
	"os"
	"syscall"
)

type desktopFileIdentity struct {
	device uint64
	inode  uint64
	links  uint64
}

func desktopExecutableFileIdentity(info os.FileInfo) (desktopFileIdentity, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return desktopFileIdentity{}, errDesktopProviderUnavailable
	}
	return desktopFileIdentity{
		device: uint64(stat.Dev),
		inode:  stat.Ino,
		links:  uint64(stat.Nlink),
	}, nil
}
