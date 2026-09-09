//go:build darwin || linux

package ompprobe

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	probeDirectoryFlags   = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	probeSourceFileFlags  = unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	probeDestinationFlags = unix.O_WRONLY | unix.O_CREAT | unix.O_EXCL | unix.O_NOFOLLOW | unix.O_CLOEXEC
)

var (
	errExportTraversal   = errors.New("probe export path is unsafe")
	errExportSource      = errors.New("probe source file is unsafe")
	errExportRead        = errors.New("read probe source file")
	errExportDestination = errors.New("probe destination directory is unsafe")
	errExportPublish     = errors.New("publish probe records")
)

// openProbeDirectory walks path one component at a time starting from the
// filesystem root. Every component is opened with O_NOFOLLOW and confirmed to
// be the same directory both through the new descriptor and through a
// no-follow stat of the name in its parent, so no ancestor can be a symlink or
// be swapped mid-walk.
func openProbeDirectory(path string) (int, error) {
	currentFD, err := unix.Open(string(filepath.Separator), probeDirectoryFlags, 0)
	if err != nil {
		return -1, errExportTraversal
	}
	components := strings.Split(strings.TrimPrefix(path, string(filepath.Separator)), string(filepath.Separator))
	for _, component := range components {
		if component == "" {
			continue
		}
		nextFD, openErr := openProbeChildDirectory(currentFD, component)
		_ = unix.Close(currentFD)
		if openErr != nil {
			return -1, openErr
		}
		currentFD = nextFD
	}
	return currentFD, nil
}

// descendProbeDirectories continues a walk relative to an already-open
// directory descriptor.
func descendProbeDirectories(parentFD int, components []string) (int, error) {
	currentFD := parentFD
	for _, component := range components {
		nextFD, err := openProbeChildDirectory(currentFD, component)
		_ = unix.Close(currentFD)
		if err != nil {
			return -1, err
		}
		currentFD = nextFD
	}
	return currentFD, nil
}

func openProbeChildDirectory(parentFD int, name string) (int, error) {
	if !validRootComponent(name) {
		return -1, errExportTraversal
	}
	fd, err := unix.Openat(parentFD, name, probeDirectoryFlags, 0)
	if err != nil {
		return -1, errExportTraversal
	}
	var opened, onPath unix.Stat_t
	if unix.Fstat(fd, &opened) != nil ||
		unix.Fstatat(parentFD, name, &onPath, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!probeDirectoryStat(opened) || !probeDirectoryStat(onPath) ||
		!sameProbeFile(opened, onPath) {
		_ = unix.Close(fd)
		return -1, errExportTraversal
	}
	return fd, nil
}

func probeDirectoryStat(stat unix.Stat_t) bool {
	return uint32(stat.Mode)&unix.S_IFMT == unix.S_IFDIR
}

// trustedProbeDestinationStat additionally requires the retained directory to
// be closed to group and world writers and owned by the runner or by the
// effective user, which is what makes exclusive creation inside it meaningful.
func trustedProbeDestinationStat(stat unix.Stat_t, ownerUID uint32) bool {
	if !probeDirectoryStat(stat) || uint32(stat.Mode)&0o022 != 0 {
		return false
	}
	return stat.Uid == ownerUID || uint64(stat.Uid) == uint64(unix.Geteuid())
}

// acceptedProbeSourceStat is the whole trust decision for the source file: a
// regular file with exactly one link, owned by the UID that ran the probe, of
// a bounded non-zero size. A hardlink to a protected file fails the link
// count, a root-owned sentinel fails the owner check, and a device, FIFO or
// directory fails the type check.
func acceptedProbeSourceStat(stat unix.Stat_t, sourceUID uint32) bool {
	return uint32(stat.Mode)&unix.S_IFMT == unix.S_IFREG &&
		uint64(stat.Nlink) == 1 && stat.Uid == sourceUID &&
		stat.Size > 0 && stat.Size <= MaxTotalBytes
}

func publishedProbeDestinationStat(stat unix.Stat_t, options ExportOptions, size int64) bool {
	return uint32(stat.Mode)&unix.S_IFMT == unix.S_IFREG &&
		uint32(stat.Mode)&0o7777 == 0o600 && uint64(stat.Nlink) == 1 &&
		stat.Uid == options.OwnerUID && stat.Gid == options.OwnerGID && stat.Size == size
}

func sameProbeFile(left, right unix.Stat_t) bool {
	return uint64(left.Dev) == uint64(right.Dev) && uint64(left.Ino) == uint64(right.Ino)
}
