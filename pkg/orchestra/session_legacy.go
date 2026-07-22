package orchestra

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

const legacySessionFilenamePrefix = "autopus-orch-session-"

func legacySessionFilename(id string) (string, error) {
	if err := validateSafeArtifactName("session ID", id); err != nil {
		return "", fmt.Errorf("invalid session ID: %w", err)
	}
	return legacySessionFilenamePrefix + id + ".json", nil
}

func openLegacySessionRoot() (*os.Root, error) {
	root, err := os.OpenRoot(os.TempDir())
	if err != nil {
		return nil, fmt.Errorf("open legacy session root: %w", err)
	}
	return root, nil
}

func readSessionEntry(root *os.Root, name, id string) (*OrchestraSession, fs.FileInfo, error) {
	initialInfo, err := root.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if !initialInfo.Mode().IsRegular() {
		return nil, nil, errors.New("session entry is not a regular file")
	}

	file, err := root.Open(name)
	if err != nil {
		return nil, nil, err
	}
	openedInfo, statErr := file.Stat()
	currentInfo, currentErr := root.Lstat(name)
	if statErr != nil || currentErr != nil || !currentInfo.Mode().IsRegular() ||
		!os.SameFile(initialInfo, openedInfo) || !os.SameFile(openedInfo, currentInfo) {
		_ = file.Close()
		return nil, nil, errors.New("session entry changed while opening")
	}

	data, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return nil, nil, readErr
	}
	if closeErr != nil {
		return nil, nil, fmt.Errorf("close session entry: %w", closeErr)
	}

	var session OrchestraSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, nil, fmt.Errorf("parse session: %w", err)
	}
	if session.ID != id {
		return nil, nil, fmt.Errorf("persisted ID %q does not match", session.ID)
	}
	return &session, openedInfo, nil
}

func loadSessionFromRoot(root *os.Root, name, id string) (*OrchestraSession, error) {
	session, _, err := readSessionEntry(root, name, id)
	if err != nil {
		return nil, fmt.Errorf("read session %s: %w", id, err)
	}
	return session, nil
}

// LoadSession reads the private session path and falls back to the legacy
// temp-root basename only when the private entry does not exist.
func LoadSession(id string) (*OrchestraSession, error) {
	name, err := sessionFilename(id)
	if err != nil {
		return nil, err
	}
	root, err := openSessionRoot(false)
	if err == nil {
		session, readErr := loadSessionFromRoot(root, name, id)
		_ = root.Close()
		if readErr == nil || !errors.Is(readErr, fs.ErrNotExist) {
			return session, readErr
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read session %s: %w", id, err)
	}

	legacyName, err := legacySessionFilename(id)
	if err != nil {
		return nil, err
	}
	legacyRoot, err := openLegacySessionRoot()
	if err != nil {
		return nil, fmt.Errorf("read session %s: %w", id, err)
	}
	defer func() { _ = legacyRoot.Close() }()
	return loadSessionFromRoot(legacyRoot, legacyName, id)
}

func removeSessionFromRoot(root *os.Root, name, id string) error {
	_, openedInfo, err := readSessionEntry(root, name, id)
	if err != nil {
		return err
	}
	currentInfo, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if !currentInfo.Mode().IsRegular() || !os.SameFile(openedInfo, currentInfo) {
		return errors.New("session entry changed before removal")
	}
	return root.Remove(name)
}

// RemoveSession removes the private session entry. When it is absent, a safe,
// matching legacy entry is removed for patch-release compatibility.
func RemoveSession(id string) error {
	name, err := sessionFilename(id)
	if err != nil {
		return err
	}
	root, err := openSessionRoot(false)
	if err == nil {
		removeErr := removeSessionFromRoot(root, name, id)
		_ = root.Close()
		if removeErr == nil {
			return nil
		}
		if !errors.Is(removeErr, fs.ErrNotExist) {
			return fmt.Errorf("remove session %s: %w", id, removeErr)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove session %s: %w", id, err)
	}

	legacyName, err := legacySessionFilename(id)
	if err != nil {
		return err
	}
	legacyRoot, err := openLegacySessionRoot()
	if err != nil {
		return fmt.Errorf("remove session %s: %w", id, err)
	}
	defer func() { _ = legacyRoot.Close() }()
	if err := removeSessionFromRoot(legacyRoot, legacyName, id); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove session %s: %w", id, err)
	}
	return nil
}

func openSessionUpdateTarget(id string) (*os.Root, string, fs.FileInfo, error) {
	name, err := sessionFilename(id)
	if err != nil {
		return nil, "", nil, err
	}
	root, err := openSessionRoot(false)
	if err == nil {
		_, info, readErr := readSessionEntry(root, name, id)
		if readErr == nil {
			return root, name, info, nil
		}
		_ = root.Close()
		if !errors.Is(readErr, fs.ErrNotExist) {
			return nil, "", nil, fmt.Errorf("read session before update: %w", readErr)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, "", nil, fmt.Errorf("open session update root: %w", err)
	}

	legacyName, err := legacySessionFilename(id)
	if err != nil {
		return nil, "", nil, err
	}
	legacyRoot, err := openLegacySessionRoot()
	if err != nil {
		return nil, "", nil, err
	}
	_, info, err := readSessionEntry(legacyRoot, legacyName, id)
	if err != nil {
		_ = legacyRoot.Close()
		return nil, "", nil, fmt.Errorf("read legacy session before update: %w", err)
	}
	return legacyRoot, legacyName, info, nil
}
