//go:build darwin || linux

package cli

import (
	"errors"

	"golang.org/x/sys/unix"
)

func (transaction *publicKeyReceiptBundleTransaction) finish() error {
	var cleanupErr error
	if !transaction.completed {
		name := transaction.stageName
		if transaction.published {
			name = transaction.output.bundleName
		}
		cleanupErr = transaction.removeOwnedBundle(name)
		if syncErr := unix.Fsync(transaction.parentFD); syncErr != nil {
			cleanupErr = errors.Join(
				cleanupErr,
				errors.New("sync public key receipt rollback directory"),
			)
		}
	}
	if transaction.stageFD >= 0 {
		if err := unix.Close(transaction.stageFD); err != nil {
			cleanupErr = errors.Join(
				cleanupErr,
				errors.New("close public key receipt staging descriptor"),
			)
		}
		transaction.stageFD = -1
	}
	if transaction.parentFD >= 0 {
		if err := unix.Close(transaction.parentFD); err != nil {
			cleanupErr = errors.Join(
				cleanupErr,
				errors.New("close public key receipt parent descriptor"),
			)
		}
		transaction.parentFD = -1
	}
	return cleanupErr
}

func (transaction *publicKeyReceiptBundleTransaction) removeOwnedBundle(name string) error {
	if name == "" {
		return nil
	}
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW |
		unix.O_NONBLOCK | unix.O_CLOEXEC
	directoryFD, err := unix.Openat(transaction.parentFD, name, flags, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		if transaction.published && name == transaction.output.bundleName {
			return errors.Join(
				errors.New("public key receipt rollback ownership changed"),
				transaction.quarantineUnexpectedPublication(name),
			)
		}
		return errors.New("open owned public key receipt rollback directory")
	}
	var directoryStat unix.Stat_t
	owned := unix.Fstat(directoryFD, &directoryStat) == nil &&
		samePublicKeyReceiptUnixFile(directoryStat, transaction.stageStat)
	if !owned {
		_ = unix.Close(directoryFD)
		if transaction.published && name == transaction.output.bundleName {
			return errors.Join(
				errors.New("public key receipt rollback ownership changed"),
				transaction.quarantineUnexpectedPublication(name),
			)
		}
		return errors.New("public key receipt rollback ownership changed")
	}
	var cleanupErr error
	for _, entry := range []string{
		publicKeyReceiptEntryName,
		publicKeyReceiptSignatureEntryName,
	} {
		if err := unix.Unlinkat(directoryFD, entry, 0); err != nil &&
			!errors.Is(err, unix.ENOENT) {
			cleanupErr = errors.Join(
				cleanupErr,
				errors.New("remove public key receipt rollback entry"),
			)
		}
	}
	if err := unix.Fsync(directoryFD); err != nil {
		cleanupErr = errors.Join(
			cleanupErr,
			errors.New("sync public key receipt rollback bundle"),
		)
	}
	if err := unix.Close(directoryFD); err != nil {
		cleanupErr = errors.Join(
			cleanupErr,
			errors.New("close public key receipt rollback bundle"),
		)
	}
	if err := unix.Unlinkat(transaction.parentFD, name, unix.AT_REMOVEDIR); err != nil &&
		!errors.Is(err, unix.ENOENT) {
		cleanupErr = errors.Join(
			cleanupErr,
			errors.New("remove public key receipt rollback bundle"),
		)
		if transaction.published && name == transaction.output.bundleName {
			cleanupErr = errors.Join(
				cleanupErr,
				transaction.quarantineUnexpectedPublication(name),
			)
		}
	}
	return cleanupErr
}

func (transaction *publicKeyReceiptBundleTransaction) quarantineUnexpectedPublication(
	name string,
) error {
	for attempt := 0; attempt < 16; attempt++ {
		quarantine, err := randomPublicKeyReceiptName(
			publicKeyReceiptStagePrefix + "quarantine-",
		)
		if err != nil {
			return errors.New("generate receipt quarantine name")
		}
		err = publishPublicKeyReceiptBundleNoReplace(
			transaction.parentFD,
			name,
			quarantine,
		)
		if err == nil || errors.Is(err, unix.ENOENT) {
			return nil
		}
		if !errors.Is(err, unix.EEXIST) {
			return errors.New("quarantine unexpected receipt publication")
		}
	}
	return errors.New("allocate receipt publication quarantine")
}
