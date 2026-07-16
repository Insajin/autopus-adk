package companionmanifest

import (
	"errors"
	"os"
	"path/filepath"
)

func validSignedPairState(state signedPairState) bool {
	return state.SchemaVersion == signedPairStateSchema &&
		validSignedPairOutputName(state.ManifestName) &&
		validSignedPairOutputName(state.SignatureName) &&
		state.ManifestName != state.SignatureName
}

func (transaction *signedPairTransaction) validateBeforeBackup() error {
	if !sameSignedPairDirectoryPath(transaction.parentPath, transaction.parentInfo) ||
		!transaction.stageIdentityMatches() ||
		!signedPairOutputMatches(
			transaction.root,
			transaction.state.ManifestName,
			transaction.state.ManifestExisted,
			transaction.manifestInfo,
		) || !signedPairOutputMatches(
		transaction.root,
		transaction.state.SignatureName,
		transaction.state.SignatureExisted,
		transaction.signatureInfo,
	) {
		return errors.New("signed pair transaction identity changed")
	}
	return nil
}

func (transaction *signedPairTransaction) stageIdentityMatches() bool {
	if transaction.stageRoot == nil {
		return false
	}
	pathInfo, pathErr := transaction.root.Lstat(transaction.stageName)
	rootInfo, rootErr := transaction.stageRoot.Stat(".")
	return pathErr == nil && rootErr == nil && pathInfo.IsDir() &&
		os.SameFile(transaction.stageInfo, pathInfo) &&
		os.SameFile(transaction.stageInfo, rootInfo)
}

func signedPairOutputMatches(
	root *os.Root,
	name string,
	existed bool,
	want os.FileInfo,
) bool {
	info, err := root.Lstat(name)
	if !existed {
		return errors.Is(err, os.ErrNotExist)
	}
	return err == nil && info.Mode().IsRegular() && want != nil && os.SameFile(want, info)
}

func (transaction *signedPairTransaction) backupOutputs() error {
	outputs := []struct {
		name    string
		backup  string
		existed bool
	}{
		{transaction.state.ManifestName, signedPairManifestOld, transaction.state.ManifestExisted},
		{transaction.state.SignatureName, signedPairSignatureOld, transaction.state.SignatureExisted},
	}
	for _, output := range outputs {
		if output.existed {
			if err := transaction.root.Rename(
				output.name,
				filepath.Join(transaction.stageName, output.backup),
			); err != nil {
				return errors.New("backup existing signed output")
			}
		}
	}
	if err := syncRootDirectory(transaction.root); err != nil {
		return err
	}
	return syncRootDirectory(transaction.stageRoot)
}

func (transaction *signedPairTransaction) rollback() error {
	if err := restoreSignedPairState(
		transaction.root,
		transaction.stageName,
		transaction.state,
	); err != nil {
		return errors.Join(err, transaction.releaseLock())
	}
	if err := transaction.detachTransactionDirectory(); err != nil {
		return errors.Join(err, transaction.releaseLock())
	}
	if err := transaction.releaseLock(); err != nil {
		return err
	}
	return transaction.removeTransactionDirectory()
}

func restoreSignedPairState(root *os.Root, transactionName string, state signedPairState) error {
	outputs := []struct {
		name    string
		backup  string
		existed bool
	}{
		{state.ManifestName, signedPairManifestOld, state.ManifestExisted},
		{state.SignatureName, signedPairSignatureOld, state.SignatureExisted},
	}
	for _, output := range outputs {
		backupPath := filepath.Join(transactionName, output.backup)
		backupInfo, backupErr := root.Lstat(backupPath)
		if output.existed && backupErr == nil && backupInfo.Mode().IsRegular() {
			if err := removeSignedPairOutput(root, output.name); err != nil {
				return err
			}
			if err := root.Rename(backupPath, output.name); err != nil {
				return errors.New("restore signed pair backup")
			}
			continue
		}
		if output.existed {
			if backupErr != nil && !errors.Is(backupErr, os.ErrNotExist) {
				return errors.New("inspect signed pair backup")
			}
			info, err := root.Lstat(output.name)
			if err != nil || !info.Mode().IsRegular() {
				return errors.New("signed pair backup and original are both absent")
			}
			continue
		}
		if backupErr == nil || !errors.Is(backupErr, os.ErrNotExist) {
			return errors.New("unexpected signed pair backup")
		}
		if err := removeSignedPairOutput(root, output.name); err != nil {
			return err
		}
	}
	return syncRootDirectory(root)
}

func removeSignedPairOutput(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("refuse to remove non-regular signed output")
	}
	return root.Remove(name)
}

func (transaction *signedPairTransaction) removeTransactionDirectory() error {
	return removeSignedPairTransactionDirectory(transaction.root, transaction.stageName)
}

func removeSignedPairTransactionDirectory(root *os.Root, name string) error {
	for _, entry := range []string{
		signedPairManifestNew,
		signedPairSignatureNew,
		signedPairManifestOld,
		signedPairSignatureOld,
		signedPairPreparedName,
		signedPairCommitName,
		signedPairStateName,
		signedPairLockName,
	} {
		if err := removeSignedPairTransactionEntry(root, filepath.Join(name, entry)); err != nil {
			return err
		}
	}
	if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.New("remove signed pair transaction directory")
	}
	return syncRootDirectory(root)
}

func removeSignedPairTransactionEntry(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("refuse to remove invalid signed pair transaction entry")
	}
	return root.Remove(name)
}

func (transaction *signedPairTransaction) closeStage() {
	if transaction.stageRoot != nil {
		_ = transaction.stageRoot.Close()
		transaction.stageRoot = nil
	}
}

func (transaction *signedPairTransaction) releaseLock() error {
	return releaseSignedPairLockFile(&transaction.lockFile)
}

func (transaction *signedPairTransaction) detachTransactionDirectory() error {
	cleanupName, err := detachSignedPairTransactionDirectory(
		transaction.root,
		transaction.stageName,
	)
	if err != nil {
		return err
	}
	transaction.stageName = cleanupName
	return nil
}

func sameSignedPairDirectoryPath(path string, want os.FileInfo) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 &&
		os.SameFile(info, want)
}

func syncRootDirectory(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return errors.New("open signed output directory for sync")
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil || closeErr != nil {
		return errors.New("sync signed output directory")
	}
	return nil
}
