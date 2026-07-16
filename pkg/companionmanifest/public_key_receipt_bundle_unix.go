//go:build darwin || linux

package companionmanifest

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type publicKeyReceiptBundleEntry struct {
	name string
	file *os.File
	stat unix.Stat_t
}

type publicKeyReceiptBundleTransaction struct {
	bundlePath string
	parentPath string
	bundleName string
	parentFD   int
	bundleFD   int
	parentStat unix.Stat_t
	bundleStat unix.Stat_t
	receipt    publicKeyReceiptBundleEntry
	signature  publicKeyReceiptBundleEntry
}

func verifyPublicKeyReceiptBundleAuthority(
	bundlePath string,
	policy PublicKeyReceiptPolicy,
	anchor publicKeyReceiptA0Anchor,
	hook publicKeyReceiptBundleHook,
) (trusted TrustedPublicKeyReceipt, err error) {
	transaction, err := openPublicKeyReceiptBundleTransaction(bundlePath)
	if err != nil {
		return TrustedPublicKeyReceipt{}, err
	}
	defer func() {
		if closeErr := transaction.close(); closeErr != nil {
			trusted = TrustedPublicKeyReceipt{}
			err = errors.Join(err, closeErr)
		}
	}()
	record, err := transaction.readRecord()
	if err != nil {
		return TrustedPublicKeyReceipt{}, err
	}
	trusted, err = verifyPublicKeyReceiptA0Record(record, policy, anchor)
	if err != nil {
		return TrustedPublicKeyReceipt{}, err
	}
	if hook != nil {
		if err := hook(publicKeyReceiptBundleAfterVerification); err != nil {
			return TrustedPublicKeyReceipt{}, err
		}
	}
	if err := transaction.recheck(record); err != nil {
		return TrustedPublicKeyReceipt{}, err
	}
	return trusted, nil
}

func openPublicKeyReceiptBundleTransaction(
	path string,
) (*publicKeyReceiptBundleTransaction, error) {
	if path == "" {
		return nil, errors.New("invalid public key receipt bundle path")
	}
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil || filepath.Base(absolute) == string(filepath.Separator) ||
		filepath.Base(absolute) == "." {
		return nil, errors.New("invalid public key receipt bundle path")
	}
	transaction := &publicKeyReceiptBundleTransaction{
		bundlePath: absolute,
		parentPath: filepath.Dir(absolute),
		bundleName: filepath.Base(absolute),
		parentFD:   -1,
		bundleFD:   -1,
	}
	if err := transaction.open(); err != nil {
		_ = transaction.close()
		return nil, err
	}
	return transaction, nil
}

func (transaction *publicKeyReceiptBundleTransaction) open() error {
	directoryFlags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW |
		unix.O_NONBLOCK | unix.O_CLOEXEC
	parentFD, err := unix.Open(transaction.parentPath, directoryFlags, 0)
	if err != nil {
		return errors.New("open public key receipt parent directory")
	}
	transaction.parentFD = parentFD
	if unix.Fstat(parentFD, &transaction.parentStat) != nil ||
		!securePublicKeyReceiptBundleParent(&transaction.parentStat) ||
		!samePublicKeyReceiptBundleDirectoryPath(
			transaction.parentPath,
			transaction.parentStat,
		) {
		return errors.New("insecure public key receipt parent directory")
	}
	bundleFD, err := unix.Openat(
		parentFD,
		transaction.bundleName,
		directoryFlags,
		0,
	)
	if err != nil {
		return errors.New("open public key receipt bundle directory")
	}
	transaction.bundleFD = bundleFD
	var pathStat unix.Stat_t
	if unix.Fstat(bundleFD, &transaction.bundleStat) != nil ||
		unix.Fstatat(parentFD, transaction.bundleName, &pathStat, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!securePublicKeyReceiptBundleDirectory(&transaction.bundleStat) ||
		!samePublicKeyReceiptBundleFile(transaction.bundleStat, pathStat) {
		return errors.New("invalid public key receipt bundle directory")
	}
	return nil
}

func (transaction *publicKeyReceiptBundleTransaction) readRecord() (
	publicKeyReceiptBundleRecord,
	error,
) {
	if err := transaction.checkExactEntryNames(); err != nil {
		return publicKeyReceiptBundleRecord{}, err
	}
	receipt, err := transaction.openEntry(publicKeyReceiptBundleEntryName)
	if err != nil {
		return publicKeyReceiptBundleRecord{}, err
	}
	transaction.receipt = receipt
	signature, err := transaction.openEntry(publicKeyReceiptBundleSignatureEntryName)
	if err != nil {
		return publicKeyReceiptBundleRecord{}, err
	}
	transaction.signature = signature
	if samePublicKeyReceiptBundleFile(receipt.stat, signature.stat) {
		return publicKeyReceiptBundleRecord{}, errors.New("duplicate public key receipt bundle entries")
	}
	receiptBytes, err := transaction.readEntry(&transaction.receipt)
	if err != nil {
		return publicKeyReceiptBundleRecord{}, err
	}
	signatureBytes, err := transaction.readEntry(&transaction.signature)
	if err != nil {
		return publicKeyReceiptBundleRecord{}, err
	}
	return publicKeyReceiptBundleRecord{
		receiptBytes: receiptBytes,
		signature:    signatureBytes,
	}, nil
}

func (transaction *publicKeyReceiptBundleTransaction) openEntry(
	name string,
) (publicKeyReceiptBundleEntry, error) {
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	fileFD, err := unix.Openat(transaction.bundleFD, name, flags, 0)
	if err != nil {
		return publicKeyReceiptBundleEntry{}, errors.New("open public key receipt bundle entry")
	}
	file := os.NewFile(uintptr(fileFD), name)
	if file == nil {
		_ = unix.Close(fileFD)
		return publicKeyReceiptBundleEntry{}, errors.New("adopt public key receipt bundle entry")
	}
	var openedStat unix.Stat_t
	var pathStat unix.Stat_t
	if unix.Fstat(fileFD, &openedStat) != nil ||
		unix.Fstatat(transaction.bundleFD, name, &pathStat, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!securePublicKeyReceiptBundleEntry(&openedStat) ||
		!securePublicKeyReceiptBundleEntry(&pathStat) ||
		!samePublicKeyReceiptBundleFile(openedStat, pathStat) {
		_ = file.Close()
		return publicKeyReceiptBundleEntry{}, errors.New("invalid public key receipt bundle entry")
	}
	return publicKeyReceiptBundleEntry{name: name, file: file, stat: openedStat}, nil
}

func (transaction *publicKeyReceiptBundleTransaction) readEntry(
	entry *publicKeyReceiptBundleEntry,
) ([]byte, error) {
	if _, err := entry.file.Seek(0, io.SeekStart); err != nil {
		return nil, errors.New("seek public key receipt bundle entry")
	}
	data, err := io.ReadAll(io.LimitReader(entry.file, maxPublicKeyReceiptBundleEntryBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxPublicKeyReceiptBundleEntryBytes {
		return nil, errors.New("read public key receipt bundle entry")
	}
	var descriptorStat unix.Stat_t
	var pathStat unix.Stat_t
	if unix.Fstat(int(entry.file.Fd()), &descriptorStat) != nil ||
		unix.Fstatat(transaction.bundleFD, entry.name, &pathStat, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!securePublicKeyReceiptBundleEntry(&descriptorStat) ||
		!samePublicKeyReceiptBundleFile(entry.stat, descriptorStat) ||
		!samePublicKeyReceiptBundleFile(entry.stat, pathStat) {
		return nil, errors.New("public key receipt bundle entry identity changed")
	}
	return data, nil
}

func (transaction *publicKeyReceiptBundleTransaction) recheck(
	record publicKeyReceiptBundleRecord,
) error {
	var parentStat unix.Stat_t
	var bundleStat unix.Stat_t
	var bundlePathStat unix.Stat_t
	if unix.Fstat(transaction.parentFD, &parentStat) != nil ||
		unix.Fstat(transaction.bundleFD, &bundleStat) != nil ||
		unix.Fstatat(
			transaction.parentFD,
			transaction.bundleName,
			&bundlePathStat,
			unix.AT_SYMLINK_NOFOLLOW,
		) != nil ||
		!securePublicKeyReceiptBundleParent(&parentStat) ||
		!securePublicKeyReceiptBundleDirectory(&bundleStat) ||
		!samePublicKeyReceiptBundleFile(transaction.parentStat, parentStat) ||
		!samePublicKeyReceiptBundleFile(transaction.bundleStat, bundleStat) ||
		!samePublicKeyReceiptBundleFile(transaction.bundleStat, bundlePathStat) ||
		!samePublicKeyReceiptBundleDirectoryPath(transaction.parentPath, parentStat) {
		return errors.New("public key receipt bundle authority changed")
	}
	if err := transaction.checkExactEntryNames(); err != nil {
		return err
	}
	receiptBytes, err := transaction.readEntry(&transaction.receipt)
	if err != nil || !bytes.Equal(receiptBytes, record.receiptBytes) {
		return errors.New("public key receipt bundle receipt changed")
	}
	signature, err := transaction.readEntry(&transaction.signature)
	if err != nil || !bytes.Equal(signature, record.signature) {
		return errors.New("public key receipt bundle signature changed")
	}
	return nil
}

func (transaction *publicKeyReceiptBundleTransaction) checkExactEntryNames() error {
	names, err := publicKeyReceiptBundleNames(transaction.bundleFD)
	if err != nil || len(names) != 2 ||
		names[0] != publicKeyReceiptBundleEntryName ||
		names[1] != publicKeyReceiptBundleSignatureEntryName {
		return errors.New("public key receipt bundle contains unexpected entries")
	}
	return nil
}

func (transaction *publicKeyReceiptBundleTransaction) close() error {
	var closeErr error
	for _, entry := range []*publicKeyReceiptBundleEntry{
		&transaction.receipt,
		&transaction.signature,
	} {
		if entry.file != nil {
			closeErr = errors.Join(closeErr, entry.file.Close())
			entry.file = nil
		}
	}
	if transaction.bundleFD >= 0 {
		closeErr = errors.Join(closeErr, unix.Close(transaction.bundleFD))
		transaction.bundleFD = -1
	}
	if transaction.parentFD >= 0 {
		closeErr = errors.Join(closeErr, unix.Close(transaction.parentFD))
		transaction.parentFD = -1
	}
	return closeErr
}
