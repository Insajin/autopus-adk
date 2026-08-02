package omp

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type OMPContextArtifact struct {
	mu            sync.Mutex
	root          string
	baseRoot      *os.Root
	directoryRoot *os.Root
	directoryInfo fs.FileInfo
	directoryName string
	cleaned       bool
}
type OMPContextArtifactReceipt struct {
	RootClass                 string `json:"root_class"`
	PreCleanupArtifactCount   int    `json:"pre_cleanup_artifact_count"`
	PostCleanupExistenceCount int    `json:"post_cleanup_existence_count"`
	CleanupStatus             string `json:"cleanup_status"`
	Reason                    string `json:"reason"`
}

func CreateOMPContextArtifact(base string, allowedRoots []string) (*OMPContextArtifact, error) {
	abs, err := filepath.Abs(base)
	if err != nil {
		return nil, fmt.Errorf("artifact_base_invalid: %w", err)
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return nil, fmt.Errorf("artifact_base_invalid: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("artifact_base_symlink")
	}
	if !info.IsDir() {
		return nil, errors.New("artifact_base_outside_allowed_roots")
	}
	baseRoot, err := openOMPContextArtifactBase(abs, info, allowedRoots)
	if err != nil {
		return nil, err
	}
	directoryName, directoryRoot, directoryInfo, err := createOMPContextArtifactDirectory(baseRoot)
	if err != nil {
		_ = baseRoot.Close()
		return nil, err
	}
	return &OMPContextArtifact{root: filepath.Join(abs, directoryName), baseRoot: baseRoot,
		directoryRoot: directoryRoot, directoryInfo: directoryInfo, directoryName: directoryName}, nil
}
func (artifact *OMPContextArtifact) WriteOneShotOverlay(content []byte) error {
	if artifact == nil {
		return errors.New("artifact_unavailable")
	}
	artifact.mu.Lock()
	defer artifact.mu.Unlock()
	if artifact.cleaned {
		return errors.New("artifact_unavailable")
	}
	if !artifact.rootBindingValid() {
		return errors.New("artifact_root_invalid")
	}
	const overlayName = "session-overlay.yml"
	file, err := artifact.directoryRoot.OpenFile(overlayName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(content)
	if writeErr == nil && written != len(content) {
		writeErr = errors.New("artifact_overlay_short_write")
	}
	writeErr = errors.Join(writeErr, file.Close())
	if writeErr != nil {
		_ = artifact.directoryRoot.Remove(overlayName)
		return writeErr
	}
	return nil
}
func (artifact *OMPContextArtifact) Cleanup(reason string) OMPContextArtifactReceipt {
	receipt := OMPContextArtifactReceipt{RootClass: "isolated_task_owned", Reason: stableOMPContextCleanupReason(reason)}
	if artifact == nil {
		receipt.CleanupStatus = "cleanup_unverified"
		receipt.PostCleanupExistenceCount = 1
		return receipt
	}
	artifact.mu.Lock()
	defer artifact.mu.Unlock()
	if artifact.cleaned {
		receipt.CleanupStatus = "cleaned"
		return receipt
	}
	receipt.PreCleanupArtifactCount = countOMPContextArtifacts(artifact.directoryRoot)
	removed := artifact.removeOwnedDirectory()
	_ = artifact.directoryRoot.Close()
	_ = artifact.baseRoot.Close()
	artifact.cleaned = true
	if removed {
		receipt.CleanupStatus = "cleaned"
		return receipt
	}
	receipt.CleanupStatus = "cleanup_unverified"
	receipt.PostCleanupExistenceCount = 1
	return receipt
}
func WithOMPContextArtifact(base string, allowedRoots []string, run func(*OMPContextArtifact) error) (OMPContextArtifactReceipt, error) {
	artifact, err := CreateOMPContextArtifact(base, allowedRoots)
	if err != nil {
		return OMPContextArtifactReceipt{RootClass: "isolated_task_owned", CleanupStatus: "not_created", Reason: "create_failed"}, err
	}
	runErr := run(artifact)
	reason := "success"
	if runErr != nil {
		reason = "abort"
	}
	receipt := artifact.Cleanup(reason)
	if receipt.PostCleanupExistenceCount != 0 && runErr == nil {
		runErr = errors.New("artifact_cleanup_unverified")
	}
	return receipt, runErr
}

// @AX:WARN [AUTO]: artifact-root admission has cyclomatic complexity 19.
// @AX:REASON [AUTO]: gocyclo reports 19 across canonical path, allowlist, symlink, permission, and same-file checks.
func openOMPContextArtifactBase(path string, pathInfo fs.FileInfo, roots []string) (*os.Root, error) {
	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if path != abs && !pathWithinOMPContextRoot(abs, path) {
			continue
		}
		relative, err := filepath.Rel(abs, path)
		if err != nil {
			continue
		}
		rootInfo, err := os.Lstat(abs)
		if err != nil {
			return nil, fmt.Errorf("artifact_allowed_root_invalid: %w", err)
		}
		if rootInfo.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("artifact_allowed_root_symlink")
		}
		if !rootInfo.IsDir() {
			continue
		}
		canonicalRoot, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return nil, fmt.Errorf("artifact_allowed_root_invalid: %w", err)
		}
		current, err := os.OpenRoot(canonicalRoot)
		if err != nil {
			return nil, fmt.Errorf("artifact_allowed_root_invalid: %w", err)
		}
		if !sameOMPContextDirectory(current, rootInfo) {
			_ = current.Close()
			return nil, errors.New("artifact_allowed_root_changed")
		}
		for _, component := range strings.Split(relative, string(filepath.Separator)) {
			if component == "" || component == "." {
				continue
			}
			next, openErr := openOMPContextRealSubdirectory(current, component)
			_ = current.Close()
			if openErr != nil {
				return nil, openErr
			}
			current = next
		}
		canonicalPath, err := filepath.EvalSymlinks(path)
		if err != nil || !pathWithinOMPContextRoot(canonicalRoot, canonicalPath) ||
			!sameOMPContextDirectory(current, pathInfo) {
			_ = current.Close()
			return nil, errors.New("artifact_base_changed_or_outside_allowed_root")
		}
		return current, nil
	}
	return nil, errors.New("artifact_base_outside_allowed_roots")
}

func openOMPContextRealSubdirectory(parent *os.Root, name string) (*os.Root, error) {
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("artifact_base_invalid: %w", err)
	}
	if before.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("artifact_base_symlink")
	}
	if !before.IsDir() {
		return nil, errors.New("artifact_base_invalid")
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("artifact_base_invalid: %w", err)
	}
	after, err := parent.Lstat(name)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !after.IsDir() ||
		!os.SameFile(before, after) || !sameOMPContextDirectory(child, after) {
		_ = child.Close()
		return nil, errors.New("artifact_base_changed")
	}
	return child, nil
}
