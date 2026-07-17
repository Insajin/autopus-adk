package selfupdate

import (
	"errors"
	"fmt"
	"io"
	"os"
)

const windowsRecoveryComplete = "autopus-selfupdate-complete-v1\n"

type windowsCommitOps struct {
	remove      func(string) error
	lstat       func(string) (os.FileInfo, error)
	move        func(string, string) error
	readFile    func(string) ([]byte, error)
	writeMarker func(string) error
}

func commitStagedBinaryWindows(
	stagePath, targetPath string,
	expected os.FileInfo,
	ops windowsCommitOps,
) error {
	currentInfo, err := ops.lstat(targetPath)
	if err != nil {
		return fmt.Errorf("inspect target binary before Windows commit: %w", err)
	}
	if !currentInfo.Mode().IsRegular() || !os.SameFile(expected, currentInfo) {
		return errors.New("target binary changed before Windows commit")
	}

	oldPath := targetPath + ".old"
	markerPath := windowsRecoveryMarkerPath(oldPath)
	if err := prepareWindowsRecoveryPath(oldPath, markerPath, ops); err != nil {
		return err
	}
	if err := ops.move(targetPath, oldPath); err != nil {
		return fmt.Errorf("move old binary to recovery path: %w", err)
	}

	movedInfo, err := ops.lstat(oldPath)
	if err != nil || !os.SameFile(expected, movedInfo) {
		cause := errors.New("target binary changed before Windows commit")
		if err != nil {
			cause = fmt.Errorf("inspect Windows recovery binary: %w", err)
		}
		return restoreWindowsTarget(oldPath, targetPath, cause, ops.move)
	}
	if err := ops.move(stagePath, targetPath); err != nil {
		return restoreWindowsTarget(oldPath, targetPath, err, ops.move)
	}
	if err := ops.remove(oldPath); err != nil {
		if markerErr := ops.writeMarker(markerPath); markerErr != nil {
			return errors.Join(
				fmt.Errorf("new binary installed but recovery binary remains at %s: %w", oldPath, err),
				fmt.Errorf("record completed Windows replacement: %w", markerErr),
			)
		}
	}
	return nil
}

func prepareWindowsRecoveryPath(oldPath, markerPath string, ops windowsCommitOps) error {
	oldInfo, oldErr := ops.lstat(oldPath)
	if errors.Is(oldErr, os.ErrNotExist) {
		return removeOrphanWindowsMarker(markerPath, ops)
	}
	if oldErr != nil {
		return fmt.Errorf("inspect Windows recovery binary: %w", oldErr)
	}
	if !oldInfo.Mode().IsRegular() {
		return fmt.Errorf("Windows recovery path is not a regular file: %s", oldPath)
	}

	markerInfo, err := ops.lstat(markerPath)
	if err != nil || !markerInfo.Mode().IsRegular() || markerInfo.Size() != int64(len(windowsRecoveryComplete)) {
		return fmt.Errorf(
			"unresolved Windows recovery binary at %s; restore or remove it before retrying",
			oldPath,
		)
	}
	marker, err := ops.readFile(markerPath)
	if err != nil || string(marker) != windowsRecoveryComplete {
		return fmt.Errorf(
			"unresolved Windows recovery binary at %s; completion marker is invalid",
			oldPath,
		)
	}
	if err := ops.remove(oldPath); err != nil {
		return fmt.Errorf("remove completed Windows recovery binary at %s: %w", oldPath, err)
	}
	if err := ops.remove(markerPath); err != nil {
		return fmt.Errorf("remove completed Windows recovery marker at %s: %w", markerPath, err)
	}
	return nil
}

func removeOrphanWindowsMarker(markerPath string, ops windowsCommitOps) error {
	if _, err := ops.lstat(markerPath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect Windows recovery marker: %w", err)
	}
	if err := ops.remove(markerPath); err != nil {
		return fmt.Errorf("remove orphan Windows recovery marker: %w", err)
	}
	return nil
}

func windowsRecoveryMarkerPath(oldPath string) string {
	return oldPath + ".autopus-complete"
}

func writeWindowsRecoveryMarker(path string) (err error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(path)
		}
	}()
	if _, err = io.WriteString(file, windowsRecoveryComplete); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func restoreWindowsTarget(
	oldPath, targetPath string,
	cause error,
	move func(string, string) error,
) error {
	if err := move(oldPath, targetPath); err != nil {
		return errors.Join(
			cause,
			fmt.Errorf(
				"restore old binary: %w; recovery binary preserved at %s",
				err,
				oldPath,
			),
		)
	}
	return cause
}
