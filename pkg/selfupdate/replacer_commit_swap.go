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

// @AX:ANCHOR [AUTO]: Preserve the atomic-swap commit contract across Darwin and Linux implementations.
// @AX:REASON [AUTO]: Nine production and regression call sites depend on pre/post-swap inode verification and rollback behavior.
// @AX:WARN [AUTO]: This commit path contains more than eight filesystem decision branches.
// @AX:REASON [AUTO]: Inode identity, directory sync, rollback, and preserved-recovery ordering must remain fail-closed.
func commitWithAtomicSwap(
	stagePath, targetPath string,
	expected os.FileInfo,
	swap func(string, string) error,
	syncDir func(string) error,
) error {
	stageDir := filepath.Dir(stagePath)
	targetDir := filepath.Dir(targetPath)
	stagedInfo, err := regularBinaryInfo(os.Lstat, stagePath, "staged")
	if err != nil {
		return fmt.Errorf("inspect staged binary before atomic commit: %w", err)
	}
	currentInfo, err := os.Lstat(targetPath)
	if err != nil {
		return fmt.Errorf("inspect target before atomic commit: %w", err)
	}
	if !currentInfo.Mode().IsRegular() || !os.SameFile(expected, currentInfo) {
		return errors.New("target binary changed before atomic commit")
	}
	if os.SameFile(stagedInfo, currentInfo) {
		return errors.New("staged and target binary resolve to the same file")
	}
	if err := syncDir(stageDir); err != nil {
		return fmt.Errorf("sync staged directory: %w", err)
	}
	if err := swap(stagePath, targetPath); err != nil {
		return err
	}

	if verifyErr := verifyAtomicSwapState(
		stagePath, targetPath, expected, stagedInfo,
	); verifyErr != nil {
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

func verifyAtomicSwapState(
	stagePath, targetPath string,
	expectedTarget, expectedStage os.FileInfo,
) error {
	committedInfo, err := regularBinaryInfo(os.Lstat, targetPath, "committed target")
	if err != nil {
		return fmt.Errorf("inspect target after atomic commit: %w", err)
	}
	if !os.SameFile(expectedStage, committedInfo) {
		return errors.New("staged binary identity changed during atomic commit")
	}

	recoveryInfo, err := os.Lstat(stagePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect recovery binary after atomic commit: %w", err)
	}
	if !recoveryInfo.Mode().IsRegular() || !os.SameFile(expectedTarget, recoveryInfo) {
		return errors.New("target binary changed before atomic commit")
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
		rollbackErr := errors.Join(cause, fmt.Errorf("swap back: %w", err))
		if info, statErr := os.Lstat(stagePath); statErr == nil && info.Mode().IsRegular() {
			return &preserveStageError{
				stagePath: stagePath,
				err:       rollbackErr,
			}
		}
		return rollbackErr
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
