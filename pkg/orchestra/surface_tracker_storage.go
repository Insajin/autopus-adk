package orchestra

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const maxTrackerFileSize = 1 << 20

var surfaceTrackerMu sync.Mutex

type trackerWritableFile interface {
	Stat() (fs.FileInfo, error)
	Chmod(os.FileMode) error
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

type trackerCommitFunc func(root *os.Root, temporary, target string) error

func openSecureTrackerRoot(dir string, create bool) (*os.Root, error) {
	if create {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create tracker directory: %w", err)
		}
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, fmt.Errorf("inspect tracker directory: %w", err)
	}
	if !trackerDirectoryInfoSecure(info) {
		return nil, errors.New("tracker directory ownership, mode, or type is insecure")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open tracker directory: %w", err)
	}
	openedInfo, err := root.Stat(".")
	if err != nil || !os.SameFile(info, openedInfo) || !trackerDirectoryInfoSecure(openedInfo) {
		_ = root.Close()
		return nil, errors.New("tracker directory changed while opening")
	}
	return root, nil
}

func trackerDirectoryInfoSecure(info fs.FileInfo) bool {
	return info != nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 &&
		trackerInfoOwnedByCurrentUser(info) && trackerModeSecure(info, 0o700)
}

func trackerFileInfoSecure(info fs.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 &&
		trackerInfoOwnedByCurrentUser(info) && trackerModeSecure(info, 0o600)
}

func readTrackedSurfacesFromRoot(root *os.Root, name string) ([]trackedSurface, fs.FileInfo, error) {
	initialInfo, err := root.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if !trackerFileInfoSecure(initialInfo) {
		return nil, nil, errors.New("tracker entry ownership, mode, or type is insecure")
	}
	if initialInfo.Size() < 0 || initialInfo.Size() > maxTrackerFileSize {
		return nil, nil, fmt.Errorf("tracker entry exceeds %d bytes", maxTrackerFileSize)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, nil, err
	}
	openedInfo, statErr := file.Stat()
	currentInfo, currentErr := root.Lstat(name)
	if statErr != nil || currentErr != nil || !trackerFileInfoSecure(openedInfo) ||
		!trackerFileInfoSecure(currentInfo) || !os.SameFile(initialInfo, openedInfo) ||
		!os.SameFile(openedInfo, currentInfo) {
		_ = file.Close()
		return nil, nil, errors.New("tracker entry changed while opening")
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxTrackerFileSize+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, nil, readErr
	}
	if closeErr != nil {
		return nil, nil, closeErr
	}
	if len(data) > maxTrackerFileSize {
		return nil, nil, fmt.Errorf("tracker entry exceeds %d bytes", maxTrackerFileSize)
	}
	return decodeTrackedSurfaces(data), openedInfo, nil
}

func readTrackedSurfaces(path string) []trackedSurface {
	surfaceTrackerMu.Lock()
	defer surfaceTrackerMu.Unlock()
	root, err := openSecureTrackerRoot(filepath.Dir(path), false)
	if err != nil {
		return nil
	}
	defer func() { _ = root.Close() }()
	tracked, _, err := readTrackedSurfacesFromRoot(root, filepath.Base(path))
	if err != nil {
		return nil
	}
	return tracked
}

func writeTrackedSurfaces(path string, tracked []trackedSurface) {
	_ = writeTrackedSurfacesWithCommit(path, tracked, nil)
}

func writeTrackedSurfacesWithCommit(path string, tracked []trackedSurface, commit trackerCommitFunc) error {
	surfaceTrackerMu.Lock()
	defer surfaceTrackerMu.Unlock()
	root, err := openSecureTrackerRoot(filepath.Dir(path), true)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return writeTrackedSurfacesToRoot(root, filepath.Base(path), tracked, commit)
}

func writeTrackedSurfacesToRoot(
	root *os.Root,
	name string,
	tracked []trackedSurface,
	commit trackerCommitFunc,
) error {
	originalInfo, statErr := root.Lstat(name)
	if statErr == nil && !trackerFileInfoSecure(originalInfo) {
		return errors.New("refusing to replace insecure tracker entry")
	}
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return statErr
	}
	if len(tracked) == 0 {
		if errors.Is(statErr, fs.ErrNotExist) {
			return nil
		}
		currentInfo, err := root.Lstat(name)
		if err != nil || !trackerFileInfoSecure(currentInfo) ||
			!os.SameFile(originalInfo, currentInfo) {
			return errors.New("tracker entry changed before removal")
		}
		if err := root.Remove(name); err != nil {
			return err
		}
		return syncTrackerRoot(root)
	}
	data := encodeTrackedSurfaces(tracked)
	if len(data) > maxTrackerFileSize {
		return fmt.Errorf("tracker replacement exceeds %d bytes", maxTrackerFileSize)
	}
	temporary := name + "." + strings.TrimPrefix(NewSessionID(), "orch-") + ".tmp"
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(temporary) }()
	if commit == nil {
		commit = func(root *os.Root, temporary, target string) error {
			currentInfo, currentErr := root.Lstat(target)
			switch {
			case statErr == nil && (currentErr != nil || !os.SameFile(originalInfo, currentInfo)):
				return errors.New("tracker entry changed before replacement")
			case errors.Is(statErr, fs.ErrNotExist) && currentErr == nil:
				return errors.New("tracker entry appeared before replacement")
			case currentErr != nil && !errors.Is(currentErr, fs.ErrNotExist):
				return currentErr
			}
			return root.Rename(temporary, target)
		}
	}
	return persistTrackerReplacement(root, temporary, name, file, data, commit)
}

func persistTrackerReplacement(
	root *os.Root,
	temporary string,
	target string,
	file trackerWritableFile,
	data []byte,
	commit trackerCommitFunc,
) error {
	rollback := func(cause error) error {
		closeErr := file.Close()
		_ = root.Remove(temporary)
		return errors.Join(cause, closeErr)
	}
	if err := file.Chmod(0o600); err != nil {
		return rollback(err)
	}
	createdInfo, err := file.Stat()
	if err != nil {
		return rollback(err)
	}
	if !trackerFileInfoSecure(createdInfo) {
		return rollback(errors.New("created tracker replacement is insecure"))
	}
	written, err := file.Write(data)
	if err == nil && written != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return rollback(err)
	}
	if err := file.Sync(); err != nil {
		return rollback(err)
	}
	if err := file.Close(); err != nil {
		_ = root.Remove(temporary)
		return err
	}
	currentInfo, err := root.Lstat(temporary)
	if err != nil || !trackerFileInfoSecure(currentInfo) ||
		!os.SameFile(createdInfo, currentInfo) {
		_ = root.Remove(temporary)
		return errors.New("tracker replacement changed before commit")
	}
	if commit == nil {
		_ = root.Remove(temporary)
		return errors.New("nil tracker commit")
	}
	if err := commit(root, temporary, target); err != nil {
		_ = root.Remove(temporary)
		return err
	}
	return syncTrackerRoot(root)
}

func syncTrackerRoot(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	if trackerDirectorySyncUnavailable(syncErr) {
		syncErr = nil
	}
	closeErr := directory.Close()
	return errors.Join(syncErr, closeErr)
}
