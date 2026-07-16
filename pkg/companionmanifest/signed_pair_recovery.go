package companionmanifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
)

func recoverSignedFileTransactions(parentPath string) (returnErr error) {
	signedPairTransactionMutex.Lock()
	defer signedPairTransactionMutex.Unlock()
	root, parentInfo, err := openSignedPairRoot(parentPath)
	if err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, root.Close())
	}()
	if err := recoverSignedFileTransactionsRoot(root); err != nil {
		return err
	}
	if !sameSignedPairDirectoryPath(parentPath, parentInfo) {
		return errors.New("signed output directory changed during recovery")
	}
	return nil
}

func recoverSignedFileTransactionsRoot(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return errors.New("open signed output directory for recovery")
	}
	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil || closeErr != nil {
		return errors.New("list signed output transactions")
	}
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].Name() < entries[right].Name()
	})
	for _, entry := range entries {
		var recoveryErr error
		switch {
		case strings.HasPrefix(entry.Name(), signedPairCleanupPrefix):
			recoveryErr = recoverSignedPairCleanup(root, entry.Name())
		case strings.HasPrefix(entry.Name(), signedPairTransactionPrefix):
			recoveryErr = recoverSignedFileTransaction(root, entry.Name())
		}
		if recoveryErr != nil {
			return recoveryErr
		}
	}
	return nil
}

func recoverSignedPairCleanup(root *os.Root, cleanupName string) (returnErr error) {
	cleanupRoot, err := openSignedPairRecoveryRoot(root, cleanupName)
	if err != nil {
		return err
	}
	var lockFile *os.File
	if _, lockStatErr := cleanupRoot.Lstat(signedPairLockName); lockStatErr == nil {
		var active bool
		lockFile, active, err = openSignedPairTransactionLock(cleanupRoot)
		if err != nil || active {
			_ = cleanupRoot.Close()
			if active {
				return nil
			}
			return err
		}
	} else if !errors.Is(lockStatErr, os.ErrNotExist) {
		_ = cleanupRoot.Close()
		return errors.New("inspect signed pair cleanup lock")
	}
	defer func() {
		returnErr = errors.Join(returnErr, releaseSignedPairLockFile(&lockFile))
	}()
	if err := validateSignedPairTransactionEntries(cleanupRoot); err != nil {
		_ = cleanupRoot.Close()
		return err
	}
	if err := cleanupRoot.Close(); err != nil {
		return errors.New("close signed pair cleanup directory")
	}
	if err := releaseSignedPairLockFile(&lockFile); err != nil {
		return err
	}
	return removeSignedPairTransactionDirectory(root, cleanupName)
}

func openSignedPairRecoveryRoot(root *os.Root, name string) (*os.Root, error) {
	return openSignedPairRecoveryRootWithHook(root, name, nil)
}

func openSignedPairRecoveryRootWithHook(
	root *os.Root,
	name string,
	postInspectHook func(),
) (*os.Root, error) {
	info, err := root.Lstat(name)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o700 {
		return nil, errors.New("invalid signed pair recovery transaction")
	}
	if postInspectHook != nil {
		postInspectHook()
	}
	recoveryRoot, err := root.OpenRoot(name)
	if err != nil {
		return nil, errors.New("open signed pair recovery transaction")
	}
	openedInfo, err := recoveryRoot.Stat(".")
	if err != nil || !os.SameFile(info, openedInfo) {
		_ = recoveryRoot.Close()
		return nil, errors.New("signed pair recovery transaction identity changed")
	}
	return recoveryRoot, nil
}

func recoverSignedFileTransaction(root *os.Root, transactionName string) (returnErr error) {
	transactionRoot, err := openSignedPairRecoveryRoot(root, transactionName)
	if err != nil {
		return err
	}
	lockFile, active, err := openSignedPairTransactionLock(transactionRoot)
	if err != nil || active {
		_ = transactionRoot.Close()
		if active {
			return errors.New("active signed pair transaction exists")
		}
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, releaseSignedPairLockFile(&lockFile))
	}()
	if err := validateSignedPairTransactionEntries(transactionRoot); err != nil {
		_ = transactionRoot.Close()
		return err
	}
	prepared, err := signedPairTransactionPrepared(transactionRoot)
	if err != nil {
		_ = transactionRoot.Close()
		return err
	}
	if !prepared {
		if err := validateUnpreparedSignedPairTransaction(transactionRoot); err != nil {
			_ = transactionRoot.Close()
			return err
		}
		if err := transactionRoot.Close(); err != nil {
			return errors.New("close unprepared signed pair transaction")
		}
		return finalizeRecoveredSignedPair(root, transactionName, &lockFile)
	}
	state, err := readSignedPairState(transactionRoot)
	committed := false
	if err == nil {
		committed, err = signedPairTransactionCommitted(transactionRoot)
	}
	closeErr := transactionRoot.Close()
	if err != nil || closeErr != nil {
		return errors.New("validate signed pair recovery transaction")
	}
	if committed {
		for _, name := range []string{state.ManifestName, state.SignatureName} {
			info, inspectErr := root.Lstat(name)
			if inspectErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
				return errors.New("committed signed pair is incomplete during recovery")
			}
		}
	} else if err := restoreSignedPairState(root, transactionName, state); err != nil {
		return err
	}
	return finalizeRecoveredSignedPair(root, transactionName, &lockFile)
}

func finalizeRecoveredSignedPair(
	root *os.Root,
	transactionName string,
	lockFile **os.File,
) error {
	cleanupName, err := detachSignedPairTransactionDirectory(root, transactionName)
	if err != nil {
		return err
	}
	if err := releaseSignedPairLockFile(lockFile); err != nil {
		return err
	}
	return removeSignedPairTransactionDirectory(root, cleanupName)
}

func readSignedPairState(root *os.Root) (signedPairState, error) {
	info, err := root.Lstat(signedPairStateName)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 ||
		info.Size() < 1 || info.Size() > 4096 {
		return signedPairState{}, errors.New("invalid signed pair transaction state")
	}
	data, err := root.ReadFile(signedPairStateName)
	if err != nil {
		return signedPairState{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var state signedPairState
	if err := decoder.Decode(&state); err != nil {
		return signedPairState{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return signedPairState{}, errors.New("signed pair transaction state has trailing data")
	}
	canonical, err := json.Marshal(state)
	if err != nil || !bytes.Equal(canonical, data) || !validSignedPairState(state) {
		return signedPairState{}, errors.New("invalid signed pair transaction state")
	}
	return state, nil
}

func signedPairTransactionPrepared(root *os.Root) (bool, error) {
	return signedPairTransactionMarker(
		root,
		signedPairPreparedName,
		[]byte("prepared\n"),
	)
}

func signedPairTransactionCommitted(root *os.Root) (bool, error) {
	return signedPairTransactionMarker(
		root,
		signedPairCommitName,
		[]byte("committed\n"),
	)
}

func signedPairTransactionMarker(root *os.Root, name string, want []byte) (bool, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return false, errors.New("invalid signed pair transaction marker")
	}
	data, err := root.ReadFile(name)
	if err != nil || !bytes.Equal(data, want) {
		return false, errors.New("invalid signed pair transaction marker")
	}
	return true, nil
}

func validateUnpreparedSignedPairTransaction(root *os.Root) error {
	for _, name := range []string{
		signedPairManifestOld,
		signedPairSignatureOld,
		signedPairCommitName,
	} {
		if _, err := root.Lstat(name); !errors.Is(err, os.ErrNotExist) {
			return errors.New("unprepared signed pair transaction contains published state")
		}
	}
	return nil
}

func validateSignedPairTransactionEntries(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil || closeErr != nil {
		return errors.New("list signed pair transaction entries")
	}
	allowed := map[string]bool{
		signedPairStateName: true, signedPairManifestNew: true,
		signedPairSignatureNew: true, signedPairManifestOld: true,
		signedPairSignatureOld: true, signedPairPreparedName: true,
		signedPairCommitName: true,
		signedPairLockName:   true,
	}
	for _, entry := range entries {
		if !allowed[entry.Name()] {
			return errors.New("unexpected signed pair transaction entry")
		}
		info, err := root.Lstat(entry.Name())
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			return errors.New("invalid signed pair transaction entry")
		}
	}
	return nil
}
