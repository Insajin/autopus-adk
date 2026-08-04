//go:build darwin || linux

package promptlayer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func openOMPContextPromotionCanonicalDirectoryV2(path string) (int, unix.Stat_t, error) {
	if !filepath.IsAbs(path) {
		return -1, unix.Stat_t{}, fmt.Errorf("OMP context promotion path must be absolute")
	}
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	currentFD, err := unix.Open(string(filepath.Separator), flags, 0)
	if err != nil {
		return -1, unix.Stat_t{}, fmt.Errorf("open OMP context promotion filesystem root")
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, string(filepath.Separator)), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		nextFD, openErr := unix.Openat(currentFD, part, flags, 0)
		if openErr != nil {
			_ = unix.Close(currentFD)
			return -1, unix.Stat_t{}, fmt.Errorf("open OMP context promotion root component")
		}
		var openedStat unix.Stat_t
		var pathStat unix.Stat_t
		if unix.Fstat(nextFD, &openedStat) != nil ||
			unix.Fstatat(currentFD, part, &pathStat, unix.AT_SYMLINK_NOFOLLOW) != nil ||
			!validOMPContextPromotionDirectoryStatV2(openedStat) ||
			!sameOMPContextPromotionUnixIdentityV2(openedStat, pathStat) {
			_ = unix.Close(nextFD)
			_ = unix.Close(currentFD)
			return -1, unix.Stat_t{}, fmt.Errorf("inspect OMP context promotion root component")
		}
		_ = unix.Close(currentFD)
		currentFD = nextFD
	}
	var stat unix.Stat_t
	if unix.Fstat(currentFD, &stat) != nil {
		_ = unix.Close(currentFD)
		return -1, unix.Stat_t{}, fmt.Errorf("inspect OMP context promotion root")
	}
	return currentFD, stat, nil
}

func openOMPContextPromotionChildDirectoryV2(
	parentFD int,
	name string,
	validator func(unix.Stat_t) bool,
) (ompContextPromotionArtifactDirectoryV2, error) {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	fd, err := unix.Openat(parentFD, name, flags, 0)
	if err != nil {
		return ompContextPromotionArtifactDirectoryV2{}, fmt.Errorf("open OMP context promotion artifact directory")
	}
	var openedStat unix.Stat_t
	var pathStat unix.Stat_t
	if unix.Fstat(fd, &openedStat) != nil ||
		unix.Fstatat(parentFD, name, &pathStat, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!validator(openedStat) || !validator(pathStat) ||
		!sameOMPContextPromotionUnixIdentityV2(openedStat, pathStat) {
		_ = unix.Close(fd)
		return ompContextPromotionArtifactDirectoryV2{}, fmt.Errorf("OMP context promotion artifact directory is unsafe")
	}
	return ompContextPromotionArtifactDirectoryV2{name: name, fd: fd, stat: openedStat}, nil
}

func openOMPContextPromotionArtifactEntryV2(
	directoryFD int,
	name string,
	limit int64,
) (ompContextPromotionArtifactEntryV2, error) {
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	fd, err := unix.Openat(directoryFD, name, flags, 0)
	if err != nil {
		return ompContextPromotionArtifactEntryV2{}, fmt.Errorf("open OMP context promotion artifact")
	}
	file := os.NewFile(uintptr(fd), name)
	if file == nil {
		_ = unix.Close(fd)
		return ompContextPromotionArtifactEntryV2{}, fmt.Errorf("adopt OMP context promotion artifact descriptor")
	}
	var openedStat unix.Stat_t
	var pathStat unix.Stat_t
	if unix.Fstat(fd, &openedStat) != nil ||
		unix.Fstatat(directoryFD, name, &pathStat, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!secureOMPContextPromotionArtifactStatV2(openedStat, limit) ||
		!secureOMPContextPromotionArtifactStatV2(pathStat, limit) ||
		!sameOMPContextPromotionUnixIdentityV2(openedStat, pathStat) {
		_ = file.Close()
		return ompContextPromotionArtifactEntryV2{}, fmt.Errorf("OMP context promotion artifact file is unsafe")
	}
	return ompContextPromotionArtifactEntryV2{name: name, file: file, stat: openedStat, limit: limit}, nil
}

func (transaction *ompContextPromotionArtifactTransactionV2) readEntry(
	entry *ompContextPromotionArtifactEntryV2,
) ([]byte, error) {
	if _, err := entry.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek OMP context promotion artifact")
	}
	body, err := io.ReadAll(io.LimitReader(entry.file, entry.limit+1))
	if err != nil || len(body) == 0 || int64(len(body)) > entry.limit || int64(len(body)) != entry.stat.Size {
		return nil, fmt.Errorf("read OMP context promotion artifact")
	}
	artifactFD := transaction.directories[len(transaction.directories)-1].fd
	var descriptorStat unix.Stat_t
	var pathStat unix.Stat_t
	if unix.Fstat(int(entry.file.Fd()), &descriptorStat) != nil ||
		unix.Fstatat(artifactFD, entry.name, &pathStat, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!secureOMPContextPromotionArtifactStatV2(descriptorStat, entry.limit) ||
		!secureOMPContextPromotionArtifactStatV2(pathStat, entry.limit) ||
		!sameOMPContextPromotionUnixIdentityV2(entry.stat, descriptorStat) ||
		!sameOMPContextPromotionUnixIdentityV2(entry.stat, pathStat) || descriptorStat.Size != entry.stat.Size {
		return nil, fmt.Errorf("OMP context promotion artifact identity changed")
	}
	return body, nil
}

func (transaction *ompContextPromotionArtifactTransactionV2) recheck() error {
	for index, directory := range transaction.directories {
		var descriptorStat unix.Stat_t
		if unix.Fstat(directory.fd, &descriptorStat) != nil ||
			!sameOMPContextPromotionUnixIdentityV2(directory.stat, descriptorStat) {
			return fmt.Errorf("OMP context promotion directory identity changed")
		}
		validator := secureOMPContextPromotionPrivateDirectoryStatV2
		if index == 0 {
			validator = secureOMPContextPromotionRootStatV2
		} else if index == 1 {
			validator = secureOMPContextPromotionMetadataDirectoryStatV2
		}
		if !validator(descriptorStat) {
			return fmt.Errorf("OMP context promotion directory authority changed")
		}
		if index > 0 {
			var pathStat unix.Stat_t
			if unix.Fstatat(transaction.directories[index-1].fd, directory.name, &pathStat, unix.AT_SYMLINK_NOFOLLOW) != nil ||
				!validator(pathStat) || !sameOMPContextPromotionUnixIdentityV2(directory.stat, pathStat) {
				return fmt.Errorf("OMP context promotion directory path changed")
			}
		}
	}
	reopenedFD, reopenedStat, err := openOMPContextPromotionCanonicalDirectoryV2(transaction.rootPath)
	if err != nil {
		return fmt.Errorf("OMP context promotion root path changed")
	}
	_ = unix.Close(reopenedFD)
	if !sameOMPContextPromotionUnixIdentityV2(transaction.directories[0].stat, reopenedStat) {
		return fmt.Errorf("OMP context promotion root path changed")
	}
	return nil
}

func (transaction *ompContextPromotionArtifactTransactionV2) close() error {
	var closeErr error
	for _, entry := range []*ompContextPromotionArtifactEntryV2{&transaction.report, &transaction.attestation} {
		if entry.file != nil {
			closeErr = errors.Join(closeErr, entry.file.Close())
			entry.file = nil
		}
	}
	for index := len(transaction.directories) - 1; index >= 0; index-- {
		if transaction.directories[index].fd >= 0 {
			closeErr = errors.Join(closeErr, unix.Close(transaction.directories[index].fd))
			transaction.directories[index].fd = -1
		}
	}
	if closeErr != nil {
		return fmt.Errorf("close OMP context promotion artifact transaction: %w", closeErr)
	}
	return nil
}
