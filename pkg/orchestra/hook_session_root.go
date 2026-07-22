package orchestra

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const hookBaseDirectoryName = "autopus"

type hookSessionStorage struct {
	mu            sync.Mutex
	baseRoot      *os.Root
	sessionRoot   *os.Root
	baseInfo      fs.FileInfo
	sessionInfo   fs.FileInfo
	sessionName   string
	ownsDirectory bool
	closed        bool
}

func openValidatedHookDirectory(parent *os.Root, name, path, kind string) (*os.Root, fs.FileInfo, error) {
	info, err := parent.Lstat(name)
	if err != nil {
		return nil, nil, fmt.Errorf("inspect %s: %w", kind, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, nil, fmt.Errorf("%s is not a secure directory", kind)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
		return nil, nil, fmt.Errorf("%s permissions are %o, want 700", kind, info.Mode().Perm())
	}
	if !surfaceDirSecure(path) {
		return nil, nil, fmt.Errorf("%s ownership or permissions are insecure", kind)
	}

	root, err := parent.OpenRoot(name)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", kind, err)
	}
	openedInfo, err := root.Stat(".")
	if err != nil {
		_ = root.Close()
		return nil, nil, fmt.Errorf("inspect opened %s: %w", kind, err)
	}
	if !os.SameFile(info, openedInfo) {
		_ = root.Close()
		return nil, nil, fmt.Errorf("%s changed while opening", kind)
	}
	return root, openedInfo, nil
}

func newHookSessionStorage(sessionID string) (*hookSessionStorage, string, error) {
	if err := validateHookSessionID(sessionID); err != nil {
		return nil, "", err
	}
	tempRoot, err := os.OpenRoot(os.TempDir())
	if err != nil {
		return nil, "", fmt.Errorf("open temp root: %w", err)
	}
	defer func() { _ = tempRoot.Close() }()

	if err := tempRoot.Mkdir(hookBaseDirectoryName, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, "", fmt.Errorf("create hook base directory: %w", err)
	}
	basePath := filepath.Join(os.TempDir(), hookBaseDirectoryName)
	baseRoot, baseInfo, err := openValidatedHookDirectory(
		tempRoot, hookBaseDirectoryName, basePath, "hook base directory",
	)
	if err != nil {
		return nil, "", err
	}

	ownsDirectory := false
	if err := baseRoot.Mkdir(sessionID, 0o700); err == nil {
		ownsDirectory = true
	} else if !errors.Is(err, fs.ErrExist) {
		_ = baseRoot.Close()
		return nil, "", fmt.Errorf("create hook session directory: %w", err)
	}
	sessionPath := filepath.Join(basePath, sessionID)
	sessionRoot, sessionInfo, err := openValidatedHookDirectory(
		baseRoot, sessionID, sessionPath, "hook session directory",
	)
	if err != nil {
		_ = baseRoot.Close()
		return nil, "", err
	}

	return &hookSessionStorage{
		baseRoot:      baseRoot,
		sessionRoot:   sessionRoot,
		baseInfo:      baseInfo,
		sessionInfo:   sessionInfo,
		sessionName:   sessionID,
		ownsDirectory: ownsDirectory,
	}, sessionPath, nil
}

func (s *hookSessionStorage) stat(name string) (fs.FileInfo, error) {
	return s.sessionRoot.Stat(name)
}

func (s *hookSessionStorage) readFile(name string) ([]byte, error) {
	return s.sessionRoot.ReadFile(name)
}

func (s *hookSessionStorage) remove(name string) error {
	return s.sessionRoot.Remove(name)
}

func (s *hookSessionStorage) writeFile(name string, data []byte, perm os.FileMode) error {
	return s.sessionRoot.WriteFile(name, data, perm)
}

func (s *hookSessionStorage) writeJSON(name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	temporary := name + ".tmp"
	if err := s.sessionRoot.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write tmp file: %w", err)
	}
	if err := s.sessionRoot.Chmod(temporary, 0o600); err != nil {
		_ = s.sessionRoot.Remove(temporary)
		return fmt.Errorf("chmod tmp file: %w", err)
	}
	if err := s.sessionRoot.Rename(temporary, name); err != nil {
		_ = s.sessionRoot.Remove(temporary)
		return fmt.Errorf("rename %s -> %s: %w", temporary, name, err)
	}
	return nil
}

func (s *HookSession) statArtifact(name string) (fs.FileInfo, error) {
	if s.storage != nil {
		return s.storage.stat(name)
	}
	return os.Stat(filepath.Join(s.sessionDir, name))
}

func (s *HookSession) readArtifact(name string) ([]byte, error) {
	if s.storage != nil {
		return s.storage.readFile(name)
	}
	return os.ReadFile(filepath.Join(s.sessionDir, name))
}

func (s *HookSession) removeArtifact(name string) error {
	if s.storage != nil {
		return s.storage.remove(name)
	}
	return os.Remove(filepath.Join(s.sessionDir, name))
}

func (s *HookSession) writeArtifact(name string, data []byte, perm os.FileMode) error {
	if s.storage != nil {
		return s.storage.writeFile(name, data, perm)
	}
	return os.WriteFile(filepath.Join(s.sessionDir, name), data, perm)
}

func (s *HookSession) writeJSONArtifact(name string, value any) error {
	if s.storage != nil {
		return s.storage.writeJSON(name, value)
	}
	return atomicWriteJSON(filepath.Join(s.sessionDir, name), value)
}

func (s *HookSession) release() {
	if s != nil && s.storage != nil {
		s.storage.release()
	}
}

func (s *hookSessionStorage) release() {
	s.close(false)
}

func (s *hookSessionStorage) cleanup() {
	s.close(true)
}

func (s *hookSessionStorage) close(removeOwned bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if removeOwned && s.ownsDirectory {
		s.removeOwnedDirectory()
	}
	_ = s.sessionRoot.Close()
	_ = s.baseRoot.Close()
}

func (s *hookSessionStorage) removeOwnedDirectory() {
	baseInfo, baseErr := s.baseRoot.Stat(".")
	currentInfo, currentErr := s.baseRoot.Lstat(s.sessionName)
	if baseErr != nil || currentErr != nil || !os.SameFile(s.baseInfo, baseInfo) ||
		!currentInfo.IsDir() || !os.SameFile(s.sessionInfo, currentInfo) {
		return
	}
	entries, err := fs.ReadDir(s.sessionRoot.FS(), ".")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if err := s.sessionRoot.RemoveAll(entry.Name()); err != nil {
			return
		}
	}
	openedInfo, openedErr := s.sessionRoot.Stat(".")
	currentInfo, currentErr = s.baseRoot.Lstat(s.sessionName)
	if openedErr != nil || currentErr != nil || !os.SameFile(s.sessionInfo, openedInfo) ||
		!currentInfo.IsDir() || !os.SameFile(s.sessionInfo, currentInfo) {
		return
	}
	_ = s.baseRoot.Remove(s.sessionName)
}
