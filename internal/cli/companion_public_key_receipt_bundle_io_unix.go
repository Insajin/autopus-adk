//go:build darwin || linux

package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"sort"
	"time"

	"golang.org/x/sys/unix"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

func writePublicKeyReceiptBundleEntry(directoryFD int, name string, data []byte) error {
	if len(data) == 0 || len(data) > maxPublicKeyReceiptBundleEntryBytes {
		return errors.New("invalid public key receipt bundle entry size")
	}
	flags := unix.O_WRONLY | unix.O_CREAT | unix.O_EXCL | unix.O_NOFOLLOW |
		unix.O_NONBLOCK | unix.O_CLOEXEC
	fileFD, err := unix.Openat(directoryFD, name, flags, 0o600)
	if err != nil {
		return errors.New("create public key receipt bundle entry")
	}
	file := os.NewFile(uintptr(fileFD), name)
	if file == nil {
		_ = unix.Close(fileFD)
		return errors.New("adopt public key receipt bundle entry")
	}
	defer file.Close()
	var openedStat unix.Stat_t
	if unix.Fstat(fileFD, &openedStat) != nil ||
		!securePublicKeyReceiptRegularStat(&openedStat) {
		return errors.New("invalid public key receipt bundle entry")
	}
	if err := writePublicKeyReceiptDescriptor(file, data); err != nil {
		return errors.New("write public key receipt bundle entry")
	}
	if err := unix.Fsync(fileFD); err != nil {
		return errors.New("sync public key receipt bundle entry")
	}
	var descriptorAfter unix.Stat_t
	var pathAfter unix.Stat_t
	if unix.Fstat(fileFD, &descriptorAfter) != nil ||
		unix.Fstatat(directoryFD, name, &pathAfter, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!securePublicKeyReceiptRegularStat(&descriptorAfter) ||
		!securePublicKeyReceiptRegularStat(&pathAfter) ||
		!samePublicKeyReceiptUnixFile(openedStat, descriptorAfter) ||
		!samePublicKeyReceiptUnixFile(openedStat, pathAfter) {
		return errors.New("public key receipt bundle entry identity changed")
	}
	if err := file.Close(); err != nil {
		return errors.New("close public key receipt bundle entry")
	}
	return nil
}

func verifyPublicKeyReceiptBundleDescriptor(
	directoryFD int,
	wantReceipt, wantSignature []byte,
) error {
	names, err := publicKeyReceiptBundleEntryNames(directoryFD)
	if err != nil || len(names) != 2 || names[0] != publicKeyReceiptEntryName ||
		names[1] != publicKeyReceiptSignatureEntryName {
		return errors.New("public key receipt bundle contains unexpected entries")
	}
	receiptBytes, err := readPublicKeyReceiptBundleEntry(
		directoryFD,
		publicKeyReceiptEntryName,
	)
	if err != nil || !bytes.Equal(receiptBytes, wantReceipt) {
		return errors.New("public key receipt bundle receipt bytes changed")
	}
	signature, err := readPublicKeyReceiptBundleEntry(
		directoryFD,
		publicKeyReceiptSignatureEntryName,
	)
	if err != nil || !bytes.Equal(signature, wantSignature) {
		return errors.New("public key receipt bundle signature bytes changed")
	}
	receipt, err := companionmanifest.ParsePublicKeyReceiptStrict(receiptBytes)
	if err != nil {
		return errors.New("parse committed public key receipt")
	}
	issuedAt, err := time.Parse(time.RFC3339, receipt.IssuedAt)
	if err != nil {
		return errors.New("parse committed public key receipt issuance")
	}
	policy := companionmanifest.PublicKeyReceiptPolicy{
		Now: issuedAt, ExpectedKeyID: receipt.KeyID,
		ExpectedHandoff:      receipt.Handoff,
		MinimumRollbackFloor: receipt.MinimumRollbackFloor,
	}
	if err := companionmanifest.CheckPublicKeyReceiptSelfConsistency(
		receiptBytes,
		signature,
		policy,
	); err != nil {
		return errors.New("reverify committed public key receipt signature")
	}
	return nil
}

func publicKeyReceiptBundleEntryNames(directoryFD int) ([]string, error) {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW |
		unix.O_NONBLOCK | unix.O_CLOEXEC
	listingFD, err := unix.Openat(directoryFD, ".", flags, 0)
	if err != nil {
		return nil, err
	}
	directory := os.NewFile(uintptr(listingFD), "receipt-bundle")
	if directory == nil {
		_ = unix.Close(listingFD)
		return nil, errors.New("adopt receipt bundle descriptor")
	}
	names, readErr := directory.Readdirnames(-1)
	closeErr := directory.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.New("read receipt bundle entries")
	}
	sort.Strings(names)
	return names, nil
}

func readPublicKeyReceiptBundleEntry(directoryFD int, name string) ([]byte, error) {
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	fileFD, err := unix.Openat(directoryFD, name, flags, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fileFD), name)
	if file == nil {
		_ = unix.Close(fileFD)
		return nil, errors.New("adopt receipt bundle entry descriptor")
	}
	defer file.Close()
	var openedStat unix.Stat_t
	var pathStat unix.Stat_t
	if unix.Fstat(fileFD, &openedStat) != nil ||
		unix.Fstatat(directoryFD, name, &pathStat, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!securePublicKeyReceiptRegularStat(&openedStat) ||
		!securePublicKeyReceiptRegularStat(&pathStat) ||
		!samePublicKeyReceiptUnixFile(openedStat, pathStat) {
		return nil, errors.New("invalid receipt bundle entry identity")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxPublicKeyReceiptBundleEntryBytes+1))
	if err != nil || len(data) > maxPublicKeyReceiptBundleEntryBytes {
		return nil, errors.New("read receipt bundle entry")
	}
	var descriptorAfter unix.Stat_t
	if unix.Fstat(fileFD, &descriptorAfter) != nil ||
		!samePublicKeyReceiptUnixFile(openedStat, descriptorAfter) ||
		!securePublicKeyReceiptRegularStat(&descriptorAfter) {
		return nil, errors.New("receipt bundle entry changed while reading")
	}
	return data, nil
}

func writePublicKeyReceiptDescriptor(file *os.File, data []byte) error {
	for len(data) > 0 {
		written, err := file.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}
