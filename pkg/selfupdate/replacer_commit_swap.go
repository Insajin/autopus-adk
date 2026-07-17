package selfupdate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type preserveStageError struct {
	stagePath string
	err       error
}

func (e *preserveStageError) Error() string {
	return fmt.Sprintf(
		"atomic rollback failed; recovery file preserved at %s: %v",
		e.stagePath,
		e.err,
	)
}

func (e *preserveStageError) Unwrap() error { return e.err }

func preservesReplacementStage(err error) bool {
	var target *preserveStageError
	return errors.As(err, &target)
}

func commitWithAtomicSwap(
	stagePath, targetPath string,
	expected os.FileInfo,
	swap func(string, string) error,
	syncDir func(string) error,
) error {
	stageDir := filepath.Dir(stagePath)
	targetDir := filepath.Dir(targetPath)
	if err := syncDir(stageDir); err != nil {
		return fmt.Errorf("sync staged directory: %w", err)
	}
	if err := swap(stagePath, targetPath); err != nil {
		return err
	}

	swappedInfo, verifyErr := os.Lstat(stagePath)
	if verifyErr == nil && !os.SameFile(expected, swappedInfo) {
		verifyErr = errors.New("target binary changed before atomic commit")
	}
	if verifyErr != nil {
		return rollbackAtomicSwap(stagePath, targetPath, verifyErr, swap, syncDir)
	}
	if err := syncDir(targetDir); err != nil {
		return rollbackAtomicSwap(stagePath, targetPath, err, swap, syncDir)
	}
	if err := syncDir(stageDir); err != nil {
		return rollbackAtomicSwap(stagePath, targetPath, err, swap, syncDir)
	}
	return nil
}

func rollbackAtomicSwap(
	stagePath, targetPath string,
	cause error,
	swap func(string, string) error,
	syncDir func(string) error,
) error {
	if err := swap(stagePath, targetPath); err != nil {
		return &preserveStageError{
			stagePath: stagePath,
			err:       errors.Join(cause, fmt.Errorf("swap back: %w", err)),
		}
	}
	stageDir := filepath.Dir(stagePath)
	targetDir := filepath.Dir(targetPath)
	if err := syncDir(targetDir); err != nil {
		cause = errors.Join(cause, fmt.Errorf("sync rollback target directory: %w", err))
	}
	if err := syncDir(stageDir); err != nil {
		cause = errors.Join(cause, fmt.Errorf("sync rollback stage directory: %w", err))
	}
	return cause
}

func syncDirectory(path string) (err error) {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := dir.Close(); err == nil {
			err = closeErr
		}
	}()
	return dir.Sync()
}
