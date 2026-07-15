//go:build darwin || linux

package cli

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"golang.org/x/sys/unix"
)

type publicKeyReceiptBundleTransaction struct {
	output     publicKeyReceiptBundleOutput
	parentFD   int
	stageFD    int
	stageName  string
	parentStat unix.Stat_t
	stageStat  unix.Stat_t
	published  bool
	completed  bool
}

func writePublicKeyReceiptBundle(
	output publicKeyReceiptBundleOutput,
	receiptBytes, signature []byte,
	faultHook func(string) error,
) (returnErr error) {
	transaction, err := beginPublicKeyReceiptBundleTransaction(output)
	if err != nil {
		return err
	}
	defer func() {
		cleanupErr := transaction.finish()
		returnErr = errors.Join(returnErr, cleanupErr)
	}()
	if err := writePublicKeyReceiptBundleEntry(
		transaction.stageFD,
		publicKeyReceiptEntryName,
		receiptBytes,
	); err != nil {
		return err
	}
	if err := runPublicKeyReceiptFault(faultHook, "receipt_file_synced"); err != nil {
		return err
	}
	if err := writePublicKeyReceiptBundleEntry(
		transaction.stageFD,
		publicKeyReceiptSignatureEntryName,
		signature,
	); err != nil {
		return err
	}
	if err := runPublicKeyReceiptFault(faultHook, "signature_file_synced"); err != nil {
		return err
	}
	if err := verifyPublicKeyReceiptBundleDescriptor(
		transaction.stageFD,
		receiptBytes,
		signature,
	); err != nil {
		return err
	}
	if err := unix.Fsync(transaction.stageFD); err != nil {
		return errors.New("sync public key receipt staging directory")
	}
	if err := runPublicKeyReceiptFault(faultHook, "staging_dir_synced"); err != nil {
		return err
	}
	if err := runPublicKeyReceiptFault(faultHook, "before_publish"); err != nil {
		return err
	}
	if err := transaction.validatePrePublish(); err != nil {
		return err
	}
	if err := runPublicKeyReceiptFault(faultHook, "publish_ready"); err != nil {
		return err
	}
	if err := publishPublicKeyReceiptBundleNoReplace(
		transaction.parentFD,
		transaction.stageName,
		transaction.output.bundleName,
	); err != nil {
		return errors.New("publish public key receipt bundle")
	}
	transaction.published = true
	if err := runPublicKeyReceiptFault(faultHook, "bundle_published"); err != nil {
		return err
	}
	if err := transaction.verifyPublished(receiptBytes, signature); err != nil {
		return err
	}
	if err := unix.Fsync(transaction.parentFD); err != nil {
		return errors.New("sync public key receipt parent directory")
	}
	if err := runPublicKeyReceiptFault(faultHook, "parent_dir_synced"); err != nil {
		return err
	}
	if err := transaction.verifyPublished(receiptBytes, signature); err != nil {
		return err
	}
	if !samePublicKeyReceiptDirectoryPath(output.parentPath, transaction.parentStat) {
		return errors.New("public key receipt parent directory changed")
	}
	transaction.completed = true
	return nil
}

func beginPublicKeyReceiptBundleTransaction(
	output publicKeyReceiptBundleOutput,
) (*publicKeyReceiptBundleTransaction, error) {
	parentFD, err := openPublicKeyReceiptDirectory(output.parentPath)
	if err != nil {
		return nil, errors.New("open public key receipt parent directory")
	}
	transaction := &publicKeyReceiptBundleTransaction{
		output: output, parentFD: parentFD, stageFD: -1,
	}
	if unix.Fstat(parentFD, &transaction.parentStat) != nil ||
		!securePublicKeyReceiptParentStat(&transaction.parentStat) ||
		!samePublicKeyReceiptDirectoryPath(output.parentPath, transaction.parentStat) {
		_ = unix.Close(parentFD)
		return nil, errors.New("public key receipt parent directory is not secure")
	}
	var existing unix.Stat_t
	err = unix.Fstatat(parentFD, output.bundleName, &existing, unix.AT_SYMLINK_NOFOLLOW)
	if err == nil || !errors.Is(err, unix.ENOENT) {
		_ = unix.Close(parentFD)
		return nil, errors.New("public key receipt bundle already exists")
	}
	if err := transaction.createStage(); err != nil {
		_ = unix.Close(parentFD)
		return nil, err
	}
	return transaction, nil
}

func (transaction *publicKeyReceiptBundleTransaction) createStage() error {
	for attempt := 0; attempt < 16; attempt++ {
		name, err := randomPublicKeyReceiptName(publicKeyReceiptStagePrefix)
		if err != nil {
			return errors.New("generate public key receipt staging name")
		}
		if err = unix.Mkdirat(transaction.parentFD, name, 0o700); err != nil {
			if errors.Is(err, unix.EEXIST) {
				continue
			}
			return errors.New("create public key receipt staging directory")
		}
		transaction.stageName = name
		flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW |
			unix.O_NONBLOCK | unix.O_CLOEXEC
		stageFD, err := unix.Openat(transaction.parentFD, name, flags, 0)
		if err != nil {
			return errors.New("open public key receipt staging directory")
		}
		transaction.stageFD = stageFD
		if unix.Fstat(stageFD, &transaction.stageStat) != nil ||
			!securePublicKeyReceiptDirectoryStat(&transaction.stageStat, 0o700) ||
			!transaction.stageNameMatchesDescriptor() {
			return errors.New("invalid public key receipt staging directory")
		}
		return nil
	}
	return errors.New("allocate public key receipt staging directory")
}

func randomPublicKeyReceiptName(prefix string) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(random), nil
}

func (transaction *publicKeyReceiptBundleTransaction) validatePrePublish() error {
	var current unix.Stat_t
	if unix.Fstat(transaction.stageFD, &current) != nil ||
		!samePublicKeyReceiptUnixFile(current, transaction.stageStat) ||
		!securePublicKeyReceiptDirectoryStat(&current, 0o700) ||
		!transaction.stageNameMatchesDescriptor() ||
		!samePublicKeyReceiptDirectoryPath(
			transaction.output.parentPath,
			transaction.parentStat,
		) {
		return errors.New("public key receipt staging identity changed")
	}
	var outputStat unix.Stat_t
	err := unix.Fstatat(
		transaction.parentFD,
		transaction.output.bundleName,
		&outputStat,
		unix.AT_SYMLINK_NOFOLLOW,
	)
	if err == nil || !errors.Is(err, unix.ENOENT) {
		return errors.New("public key receipt output became occupied")
	}
	return nil
}

func (transaction *publicKeyReceiptBundleTransaction) stageNameMatchesDescriptor() bool {
	var pathStat unix.Stat_t
	return unix.Fstatat(
		transaction.parentFD,
		transaction.stageName,
		&pathStat,
		unix.AT_SYMLINK_NOFOLLOW,
	) == nil && samePublicKeyReceiptUnixFile(pathStat, transaction.stageStat)
}

func (transaction *publicKeyReceiptBundleTransaction) verifyPublished(
	receiptBytes, signature []byte,
) error {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW |
		unix.O_NONBLOCK | unix.O_CLOEXEC
	publishedFD, err := unix.Openat(
		transaction.parentFD,
		transaction.output.bundleName,
		flags,
		0,
	)
	if err != nil {
		return errors.New("open published public key receipt bundle")
	}
	defer unix.Close(publishedFD)
	var publishedStat unix.Stat_t
	if unix.Fstat(publishedFD, &publishedStat) != nil ||
		!securePublicKeyReceiptDirectoryStat(&publishedStat, 0o700) ||
		!samePublicKeyReceiptUnixFile(publishedStat, transaction.stageStat) {
		return errors.New("published public key receipt bundle identity changed")
	}
	return verifyPublicKeyReceiptBundleDescriptor(publishedFD, receiptBytes, signature)
}
