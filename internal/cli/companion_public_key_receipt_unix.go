//go:build darwin || linux

package cli

import (
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type publicKeyReceiptUnixIdentity struct {
	device uint64
	inode  uint64
}

func readPublicKeyReceiptSigningKey(
	path string,
	faultHook func(string) error,
) (ed25519.PrivateKey, error) {
	parentPath, base := filepath.Dir(path), filepath.Base(path)
	if base == "." || base == string(filepath.Separator) {
		return nil, errors.New("invalid receipt signing key path")
	}
	parentFD, err := openPublicKeyReceiptDirectory(parentPath)
	if err != nil {
		return nil, errors.New("open receipt signing key directory")
	}
	defer unix.Close(parentFD)
	var parentStat unix.Stat_t
	if err := unix.Fstat(parentFD, &parentStat); err != nil {
		return nil, errors.New("inspect receipt signing key directory")
	}
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	keyFD, err := unix.Openat(parentFD, base, flags, 0)
	if err != nil {
		return nil, errors.New("open receipt signing key")
	}
	file := os.NewFile(uintptr(keyFD), "receipt-signing-key")
	if file == nil {
		_ = unix.Close(keyFD)
		return nil, errors.New("adopt receipt signing key descriptor")
	}
	defer file.Close()
	var openedStat unix.Stat_t
	if err := unix.Fstat(keyFD, &openedStat); err != nil ||
		!securePublicKeyReceiptRegularStat(&openedStat) {
		return nil, errors.New("signing key must be owned regular 0600 single-link file")
	}
	if err := runPublicKeyReceiptFault(faultHook, "key_opened"); err != nil {
		return nil, err
	}
	privateKey, err := readPrivateKey(file)
	if err != nil {
		return nil, err
	}
	if err := verifyPublicKeyReceiptKeyIdentity(
		parentFD,
		base,
		parentPath,
		parentStat,
		keyFD,
		openedStat,
	); err != nil {
		clear(privateKey)
		return nil, err
	}
	if err := file.Close(); err != nil {
		clear(privateKey)
		return nil, errors.New("close receipt signing key")
	}
	return privateKey, nil
}

func verifyPublicKeyReceiptKeyIdentity(
	parentFD int,
	base, parentPath string,
	parentStat unix.Stat_t,
	keyFD int,
	openedStat unix.Stat_t,
) error {
	var descriptorAfter unix.Stat_t
	var pathAfter unix.Stat_t
	if unix.Fstat(keyFD, &descriptorAfter) != nil ||
		unix.Fstatat(parentFD, base, &pathAfter, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!securePublicKeyReceiptRegularStat(&descriptorAfter) ||
		!securePublicKeyReceiptRegularStat(&pathAfter) ||
		!samePublicKeyReceiptUnixFile(openedStat, descriptorAfter) ||
		!samePublicKeyReceiptUnixFile(openedStat, pathAfter) ||
		!samePublicKeyReceiptDirectoryPath(parentPath, parentStat) {
		return errors.New("receipt signing key identity changed")
	}
	return nil
}

func openPublicKeyReceiptDirectory(path string) (int, error) {
	return unix.Open(
		path,
		unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC,
		0,
	)
}

func samePublicKeyReceiptDirectoryPath(path string, want unix.Stat_t) bool {
	fd, err := openPublicKeyReceiptDirectory(path)
	if err != nil {
		return false
	}
	defer unix.Close(fd)
	var got unix.Stat_t
	return unix.Fstat(fd, &got) == nil && samePublicKeyReceiptUnixFile(want, got)
}

func securePublicKeyReceiptRegularStat(stat *unix.Stat_t) bool {
	return uint32(stat.Mode)&unix.S_IFMT == unix.S_IFREG &&
		uint32(stat.Mode)&0o7777 == 0o600 &&
		uint64(stat.Nlink) == 1 &&
		uint64(stat.Uid) == uint64(unix.Geteuid())
}

func securePublicKeyReceiptDirectoryStat(stat *unix.Stat_t, exactMode uint32) bool {
	mode := uint32(stat.Mode)
	return mode&unix.S_IFMT == unix.S_IFDIR && mode&0o7777 == exactMode &&
		uint64(stat.Nlink) > 0 && uint64(stat.Uid) == uint64(unix.Geteuid())
}

func securePublicKeyReceiptParentStat(stat *unix.Stat_t) bool {
	mode := uint32(stat.Mode)
	return mode&unix.S_IFMT == unix.S_IFDIR && mode&0o022 == 0 &&
		uint64(stat.Nlink) > 0 && uint64(stat.Uid) == uint64(unix.Geteuid())
}

func samePublicKeyReceiptUnixFile(left, right unix.Stat_t) bool {
	return publicKeyReceiptUnixIdentityOf(left) == publicKeyReceiptUnixIdentityOf(right)
}

func publicKeyReceiptUnixIdentityOf(stat unix.Stat_t) publicKeyReceiptUnixIdentity {
	return publicKeyReceiptUnixIdentity{
		device: uint64(stat.Dev),
		inode:  uint64(stat.Ino),
	}
}
