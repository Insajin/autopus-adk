//go:build darwin || linux

package cli

import (
	"errors"

	"golang.org/x/sys/unix"
)

func recoverStalePublicKeyReceiptStages(parentFD int) error {
	names, err := publicKeyReceiptBundleEntryNames(parentFD)
	if err != nil {
		return errors.New("list public key receipt staging directories")
	}
	removed := false
	for _, name := range names {
		if !exactPublicKeyReceiptStageName(name) {
			continue
		}
		if err := recoverStalePublicKeyReceiptStage(parentFD, name); err != nil {
			return err
		}
		removed = true
	}
	if removed && unix.Fsync(parentFD) != nil {
		return errors.New("sync recovered public key receipt parent directory")
	}
	return nil
}

func exactPublicKeyReceiptStageName(name string) bool {
	if len(name) != len(publicKeyReceiptStagePrefix)+32 ||
		name[:len(publicKeyReceiptStagePrefix)] != publicKeyReceiptStagePrefix {
		return false
	}
	for _, character := range name[len(publicKeyReceiptStagePrefix):] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func recoverStalePublicKeyReceiptStage(parentFD int, name string) error {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW |
		unix.O_NONBLOCK | unix.O_CLOEXEC
	stageFD, err := unix.Openat(parentFD, name, flags, 0)
	if err != nil {
		return errors.New("open stale public key receipt stage")
	}
	defer func() {
		if stageFD >= 0 {
			_ = unix.Close(stageFD)
		}
	}()
	var stageStat unix.Stat_t
	if unix.Fstat(stageFD, &stageStat) != nil ||
		!securePublicKeyReceiptDirectoryStat(&stageStat, 0o700) {
		return errors.New("invalid stale public key receipt stage")
	}
	if err := unix.Flock(stageFD, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return errors.New("active public key receipt transaction exists")
		}
		return errors.New("lock stale public key receipt stage")
	}
	names, err := publicKeyReceiptBundleEntryNames(stageFD)
	if err != nil {
		return errors.New("list stale public key receipt stage")
	}
	for _, entry := range names {
		if entry != publicKeyReceiptEntryName &&
			entry != publicKeyReceiptSignatureEntryName {
			return errors.New("stale public key receipt stage contains unexpected entry")
		}
		var entryStat unix.Stat_t
		if unix.Fstatat(stageFD, entry, &entryStat, unix.AT_SYMLINK_NOFOLLOW) != nil ||
			!securePublicKeyReceiptRegularStat(&entryStat) {
			return errors.New("invalid stale public key receipt stage entry")
		}
	}
	for _, entry := range names {
		if err := unix.Unlinkat(stageFD, entry, 0); err != nil {
			return errors.New("remove stale public key receipt stage entry")
		}
	}
	if unix.Fsync(stageFD) != nil {
		return errors.New("sync stale public key receipt stage")
	}
	if err := unix.Close(stageFD); err != nil {
		return errors.New("close stale public key receipt stage")
	}
	stageFD = -1
	if err := unix.Unlinkat(parentFD, name, unix.AT_REMOVEDIR); err != nil {
		return errors.New("remove stale public key receipt stage")
	}
	return nil
}
