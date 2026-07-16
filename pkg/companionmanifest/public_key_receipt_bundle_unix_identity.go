//go:build darwin || linux

package companionmanifest

import (
	"errors"
	"os"
	"sort"

	"golang.org/x/sys/unix"
)

func publicKeyReceiptBundleNames(directoryFD int) ([]string, error) {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW |
		unix.O_NONBLOCK | unix.O_CLOEXEC
	listingFD, err := unix.Openat(directoryFD, ".", flags, 0)
	if err != nil {
		return nil, err
	}
	directory := os.NewFile(uintptr(listingFD), "receipt-bundle")
	if directory == nil {
		_ = unix.Close(listingFD)
		return nil, errors.New("adopt public key receipt bundle directory")
	}
	names, readErr := directory.Readdirnames(-1)
	closeErr := directory.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.New("read public key receipt bundle directory")
	}
	sort.Strings(names)
	return names, nil
}

func samePublicKeyReceiptBundleDirectoryPath(path string, want unix.Stat_t) bool {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW |
		unix.O_NONBLOCK | unix.O_CLOEXEC
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		return false
	}
	defer func() { _ = unix.Close(fd) }()
	var got unix.Stat_t
	return unix.Fstat(fd, &got) == nil && samePublicKeyReceiptBundleFile(want, got)
}

func securePublicKeyReceiptBundleParent(stat *unix.Stat_t) bool {
	mode := uint32(stat.Mode)
	return mode&unix.S_IFMT == unix.S_IFDIR && mode&0o022 == 0 &&
		uint64(stat.Nlink) > 0 && uint64(stat.Uid) == uint64(unix.Geteuid())
}

func securePublicKeyReceiptBundleDirectory(stat *unix.Stat_t) bool {
	mode := uint32(stat.Mode)
	return mode&unix.S_IFMT == unix.S_IFDIR && mode&0o7777 == 0o700 &&
		uint64(stat.Nlink) > 0 && uint64(stat.Uid) == uint64(unix.Geteuid())
}

func securePublicKeyReceiptBundleEntry(stat *unix.Stat_t) bool {
	mode := uint32(stat.Mode)
	return mode&unix.S_IFMT == unix.S_IFREG && mode&0o7777 == 0o600 &&
		uint64(stat.Nlink) == 1 && uint64(stat.Uid) == uint64(unix.Geteuid())
}

func samePublicKeyReceiptBundleFile(left, right unix.Stat_t) bool {
	return uint64(left.Dev) == uint64(right.Dev) && uint64(left.Ino) == uint64(right.Ino)
}
