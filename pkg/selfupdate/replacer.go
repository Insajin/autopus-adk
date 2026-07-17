package selfupdate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Replacer stages a verified binary before committing it to the install path.
// Darwin and Linux commit with atomic exchange when supported; Windows
// preserves a recovery binary beside the target during its two-step commit.
type Replacer struct{}

type stagedBinaryFile interface {
	Sync() error
	Close() error
}

type replaceOps struct {
	lstat             func(string) (os.FileInfo, error)
	mkdirTemp         func(string, string) (string, error)
	rename            func(string, string) error
	copyFile          func(string, string) error
	chmod             func(string, os.FileMode) error
	openStage         func(string) (stagedBinaryFile, error)
	clearUpdateXattrs func(string) error
	commit            func(string, string, os.FileInfo) error
	removeAll         func(string) error
}

// NewReplacer creates a new Replacer.
func NewReplacer() *Replacer {
	return &Replacer{}
}

// Replace safely replaces the target binary with the new one.
func (r *Replacer) Replace(newBinaryPath, targetPath string) error {
	return replaceWithOps(newBinaryPath, targetPath, defaultReplaceOps())
}

func defaultReplaceOps() replaceOps {
	return replaceOps{
		lstat:     os.Lstat,
		mkdirTemp: os.MkdirTemp,
		rename:    os.Rename,
		copyFile:  copyReplacementFile,
		chmod:     os.Chmod,
		openStage: func(path string) (stagedBinaryFile, error) {
			return os.OpenFile(path, os.O_RDWR, 0)
		},
		clearUpdateXattrs: clearUpdateXattrs,
		commit:            commitStagedBinary,
		removeAll:         os.RemoveAll,
	}
}

func replaceWithOps(newBinaryPath, targetPath string, ops replaceOps) error {
	targetInfo, err := regularBinaryInfo(ops.lstat, targetPath, "target")
	if err != nil {
		return err
	}
	sourceInfo, err := regularBinaryInfo(ops.lstat, newBinaryPath, "new")
	if err != nil {
		return fmt.Errorf("새 바이너리 교체 실패: %w", err)
	}
	if os.SameFile(sourceInfo, targetInfo) {
		return errors.New("new and target binary resolve to the same file")
	}

	stageDir, err := ops.mkdirTemp(
		filepath.Dir(targetPath),
		"."+filepath.Base(targetPath)+".update-*",
	)
	if err != nil {
		return fmt.Errorf("prepare replacement directory: %w", err)
	}
	cleanupStage := true
	defer func() {
		if cleanupStage {
			_ = ops.removeAll(stageDir)
		}
	}()

	stagePath := filepath.Join(stageDir, filepath.Base(targetPath))
	if err := ops.rename(newBinaryPath, stagePath); err != nil {
		if !isCrossDeviceError(err) {
			return fmt.Errorf("새 바이너리 교체 실패: %w", err)
		}
		if err := ops.copyFile(newBinaryPath, stagePath); err != nil {
			return fmt.Errorf("새 바이너리 교체 실패: %w", err)
		}
	}
	stagedInfo, err := regularBinaryInfo(ops.lstat, stagePath, "staged")
	if err != nil {
		return err
	}
	if os.SameFile(stagedInfo, targetInfo) {
		return errors.New("staged and target binary resolve to the same file")
	}

	stageFile, err := prepareStagedBinary(stagePath, targetInfo.Mode().Perm(), ops)
	if err != nil {
		return err
	}
	if err := stageFile.Close(); err != nil {
		return fmt.Errorf("close staged binary: %w", err)
	}

	currentInfo, err := regularBinaryInfo(ops.lstat, targetPath, "target")
	if err != nil {
		return fmt.Errorf("recheck target binary: %w", err)
	}
	if !os.SameFile(targetInfo, currentInfo) {
		return errors.New("target binary changed during replacement")
	}
	if err := ops.commit(stagePath, targetPath, targetInfo); err != nil {
		if preservesReplacementStage(err) {
			cleanupStage = false
		}
		return fmt.Errorf("commit new binary: %w", err)
	}
	return nil
}

func regularBinaryInfo(
	lstat func(string) (os.FileInfo, error),
	path, label string,
) (os.FileInfo, error) {
	info, err := lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s binary is not a regular file", label)
	}
	return info, nil
}

func prepareStagedBinary(
	stagePath string,
	targetMode os.FileMode,
	ops replaceOps,
) (file stagedBinaryFile, err error) {
	if err := ops.chmod(stagePath, 0700); err != nil {
		return nil, fmt.Errorf("prepare staged binary access: %w", err)
	}
	file, err = ops.openStage(stagePath)
	if err != nil {
		return nil, fmt.Errorf("open staged binary: %w", err)
	}
	defer func(openedFile stagedBinaryFile) {
		if err != nil {
			_ = openedFile.Close()
		}
	}(file)
	if err = ops.chmod(stagePath, targetMode); err != nil {
		return nil, fmt.Errorf("prepare new binary permissions: %w", err)
	}
	if err = ops.clearUpdateXattrs(stagePath); err != nil {
		return nil, fmt.Errorf("prepare new binary attributes: %w", err)
	}
	if err = file.Sync(); err != nil {
		return nil, fmt.Errorf("sync staged binary: %w", err)
	}
	return file, nil
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
