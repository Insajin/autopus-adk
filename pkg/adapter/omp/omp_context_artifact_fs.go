package omp

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func sameOMPContextDirectory(root *os.Root, expected fs.FileInfo) bool {
	opened, err := root.Stat(".")
	return err == nil && opened.IsDir() && os.SameFile(expected, opened)
}

// @AX:WARN [AUTO]: exclusive artifact-directory creation has cyclomatic complexity 15.
// @AX:REASON [AUTO]: gocyclo reports 15 across entropy, collision retry, rooted-open, permission, and cleanup branches.
func createOMPContextArtifactDirectory(base *os.Root) (string, *os.Root, fs.FileInfo, error) {
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-003: 100 attempts bound collision retries for random one-shot artifact names.
	for range 100 {
		var entropy [12]byte
		if _, err := rand.Read(entropy[:]); err != nil {
			return "", nil, nil, fmt.Errorf("artifact_name_failed: %w", err)
		}
		name := "autopus-omp-context-" + hex.EncodeToString(entropy[:])
		if err := base.Mkdir(name, 0o700); errors.Is(err, fs.ErrExist) {
			continue
		} else if err != nil {
			return "", nil, nil, err
		}
		info, err := base.Lstat(name)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			_ = base.RemoveAll(name)
			return "", nil, nil, errors.New("artifact_root_invalid")
		}
		directory, err := base.OpenRoot(name)
		current, currentErr := base.Lstat(name)
		if err != nil || currentErr != nil || current.Mode()&os.ModeSymlink != 0 ||
			!current.IsDir() || !os.SameFile(info, current) || !sameOMPContextDirectory(directory, current) {
			if directory != nil {
				_ = directory.Close()
			}
			_ = base.RemoveAll(name)
			return "", nil, nil, errors.New("artifact_root_changed")
		}
		return name, directory, info, nil
	}
	return "", nil, nil, errors.New("artifact_name_exhausted")
}

func (artifact *OMPContextArtifact) rootBindingValid() bool {
	if artifact.baseRoot == nil || artifact.directoryRoot == nil || artifact.directoryInfo == nil {
		return false
	}
	current, err := artifact.baseRoot.Lstat(artifact.directoryName)
	return err == nil && current.IsDir() && current.Mode()&os.ModeSymlink == 0 &&
		os.SameFile(artifact.directoryInfo, current) &&
		sameOMPContextDirectory(artifact.directoryRoot, current)
}

func (artifact *OMPContextArtifact) removeOwnedDirectory() bool {
	if !artifact.rootBindingValid() {
		return false
	}
	entries, err := fs.ReadDir(artifact.directoryRoot.FS(), ".")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if err := artifact.directoryRoot.RemoveAll(entry.Name()); err != nil {
			return false
		}
	}
	if !artifact.rootBindingValid() || artifact.baseRoot.Remove(artifact.directoryName) != nil {
		return false
	}
	_, err = artifact.baseRoot.Lstat(artifact.directoryName)
	return errors.Is(err, fs.ErrNotExist)
}

func pathWithinOMPContextRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func countOMPContextArtifacts(root *os.Root) int {
	count := 0
	_ = fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err == nil && path != "." {
			count++
		}
		return nil
	})
	return count
}

func stableOMPContextCleanupReason(reason string) string {
	switch reason {
	case "success", "abort", "fallback", "rollback", "canary":
		return reason
	default:
		return "abort"
	}
}
