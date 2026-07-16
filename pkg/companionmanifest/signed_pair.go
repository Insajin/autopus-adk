package companionmanifest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

const (
	signedPairTransactionPrefix = ".companion-manifest.txn-"
	signedPairStateName         = "state.json"
	signedPairManifestNew       = "manifest.new"
	signedPairSignatureNew      = "signature.new"
	signedPairManifestOld       = "manifest.old"
	signedPairSignatureOld      = "signature.old"
	signedPairPreparedName      = "prepared"
	signedPairCommitName        = "committed"
	signedPairLockName          = "lock"
	signedPairStateSchema       = "adk-companion-signed-pair-transaction.v1"
	signedPairCleanupPrefix     = ".companion-manifest.cleanup-"
)

var signedPairTransactionMutex sync.Mutex

type signedPairState struct {
	SchemaVersion    string `json:"schema_version"`
	ManifestName     string `json:"manifest_name"`
	SignatureName    string `json:"signature_name"`
	ManifestExisted  bool   `json:"manifest_existed"`
	SignatureExisted bool   `json:"signature_existed"`
}

type signedPairTransaction struct {
	root          *os.Root
	stageRoot     *os.Root
	lockFile      *os.File
	parentPath    string
	parentInfo    os.FileInfo
	stageName     string
	stageInfo     os.FileInfo
	state         signedPairState
	manifestInfo  os.FileInfo
	signatureInfo os.FileInfo
}

func writeSignedFilesWithFault(
	manifestPath, signaturePath string,
	manifest, signature []byte,
	faultHook func(string) error,
) (returnErr error) {
	signedPairTransactionMutex.Lock()
	defer signedPairTransactionMutex.Unlock()

	parentPath, manifestName, signatureName, err := resolveSignedPairPaths(
		manifestPath,
		signaturePath,
	)
	if err != nil {
		return err
	}
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
		return errors.New("signed output directory changed")
	}
	manifestExisted, manifestInfo, err := inspectSignedPairOutput(root, manifestName)
	if err != nil {
		return err
	}
	signatureExisted, signatureInfo, err := inspectSignedPairOutput(root, signatureName)
	if err != nil {
		return err
	}
	transaction, err := beginSignedPairTransaction(
		root,
		parentPath,
		parentInfo,
		signedPairState{
			SchemaVersion: signedPairStateSchema, ManifestName: manifestName,
			SignatureName: signatureName, ManifestExisted: manifestExisted,
			SignatureExisted: signatureExisted,
		},
		manifestInfo,
		signatureInfo,
		manifest,
		signature,
		faultHook,
	)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		transaction.closeStage()
		if !committed {
			returnErr = errors.Join(returnErr, transaction.rollback())
		}
		returnErr = errors.Join(returnErr, transaction.releaseLock())
	}()
	if err := runSignedPairFault(faultHook, "staged"); err != nil {
		return err
	}
	if err := transaction.validateBeforeBackup(); err != nil {
		return err
	}
	if err := transaction.backupOutputs(); err != nil {
		return err
	}
	if err := runSignedPairFault(faultHook, "backed_up"); err != nil {
		return err
	}
	if !sameSignedPairDirectoryPath(parentPath, parentInfo) {
		return errors.New("signed output directory changed before publication")
	}
	if err := root.Rename(
		filepath.Join(transaction.stageName, signedPairManifestNew),
		manifestName,
	); err != nil {
		return errors.New("publish signed manifest")
	}
	if err := syncRootDirectory(root); err != nil {
		return err
	}
	if err := runSignedPairFault(faultHook, "manifest_published"); err != nil {
		return err
	}
	if !sameSignedPairDirectoryPath(parentPath, parentInfo) {
		return errors.New("signed output directory changed during publication")
	}
	if err := root.Rename(
		filepath.Join(transaction.stageName, signedPairSignatureNew),
		signatureName,
	); err != nil {
		return errors.New("publish detached signature")
	}
	if err := syncRootDirectory(root); err != nil {
		return err
	}
	if err := runSignedPairFault(faultHook, "signature_published"); err != nil {
		return err
	}
	if err := verifySignedPairOutput(root, manifestName, manifest); err != nil {
		return err
	}
	if err := verifySignedPairOutput(root, signatureName, signature); err != nil {
		return err
	}
	if !sameSignedPairDirectoryPath(parentPath, parentInfo) {
		return errors.New("signed output directory changed after publication")
	}
	if err := writeSignedPairRootFile(
		transaction.stageRoot,
		signedPairCommitName,
		[]byte("committed\n"),
	); err != nil {
		return errors.New("record committed signed output pair")
	}
	if err := syncRootDirectory(transaction.stageRoot); err != nil {
		return err
	}
	committed = true
	if err := runSignedPairFault(faultHook, "pair_committed"); err != nil {
		transaction.closeStage()
		if detachErr := transaction.detachTransactionDirectory(); detachErr != nil {
			return errors.Join(err, detachErr)
		}
		releaseErr := transaction.releaseLock()
		cleanupErr := transaction.removeTransactionDirectory()
		return errors.Join(err, releaseErr, cleanupErr)
	}
	transaction.closeStage()
	if err := transaction.detachTransactionDirectory(); err != nil {
		return err
	}
	if err := runSignedPairFault(faultHook, "transaction_detached"); err != nil {
		return err
	}
	if err := transaction.releaseLock(); err != nil {
		return err
	}
	return transaction.removeTransactionDirectory()
}

func beginSignedPairTransaction(
	root *os.Root,
	parentPath string,
	parentInfo os.FileInfo,
	state signedPairState,
	manifestInfo, signatureInfo os.FileInfo,
	manifest, signature []byte,
	faultHook func(string) error,
) (*signedPairTransaction, error) {
	name, err := createSignedPairTransactionDirectory(root, state)
	if err != nil {
		return nil, err
	}
	stageRoot, err := root.OpenRoot(name)
	if err != nil {
		_ = root.Remove(name)
		return nil, errors.New("open signed pair transaction directory")
	}
	stageInfo, err := stageRoot.Stat(".")
	if err != nil || !stageInfo.IsDir() || stageInfo.Mode().Perm() != 0o700 {
		closeErr := stageRoot.Close()
		removeErr := root.Remove(name)
		return nil, errors.Join(
			errors.New("invalid signed pair transaction directory"),
			closeErr,
			removeErr,
		)
	}
	lockFile, err := createSignedPairTransactionLock(stageRoot)
	if err != nil {
		closeErr := stageRoot.Close()
		lockRemoveErr := root.Remove(filepath.Join(name, signedPairLockName))
		directoryRemoveErr := root.Remove(name)
		return nil, errors.Join(
			errors.New("lock signed pair transaction directory"),
			closeErr,
			lockRemoveErr,
			directoryRemoveErr,
		)
	}
	transaction := &signedPairTransaction{
		root: root, stageRoot: stageRoot, lockFile: lockFile, parentPath: parentPath,
		parentInfo: parentInfo, stageName: name, stageInfo: stageInfo, state: state,
		manifestInfo: manifestInfo, signatureInfo: signatureInfo,
	}
	stateBytes, err := json.Marshal(state)
	if err != nil || writeSignedPairRootFile(stageRoot, signedPairStateName, stateBytes) != nil ||
		writeSignedPairRootFile(stageRoot, signedPairManifestNew, manifest) != nil ||
		writeSignedPairRootFile(stageRoot, signedPairSignatureNew, signature) != nil ||
		syncRootDirectory(stageRoot) != nil {
		return nil, abortSignedPairBegin(
			transaction,
			errors.New("stage signed output pair"),
		)
	}
	if err := runSignedPairFault(faultHook, "before_prepared"); err != nil {
		return nil, abortSignedPairBegin(transaction, err)
	}
	if err := writeSignedPairRootFile(
		stageRoot,
		signedPairPreparedName,
		[]byte("prepared\n"),
	); err != nil || syncRootDirectory(stageRoot) != nil {
		return nil, abortSignedPairBegin(
			transaction,
			errors.New("prepare signed output pair"),
		)
	}
	return transaction, nil
}

func abortSignedPairBegin(transaction *signedPairTransaction, cause error) error {
	transaction.closeStage()
	return errors.Join(cause, transaction.rollback())
}
