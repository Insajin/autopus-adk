package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func createWorkflowContextManagedRuntimeLease(root *os.Root) error {
	marker, err := root.OpenFile(
		workflowContextManagedRuntimeMarker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600,
	)
	if err != nil {
		return fmt.Errorf("acquire exclusive runtime lease: %w", err)
	}
	written, writeErr := marker.Write([]byte("autopus-owned\n"))
	closeErr := marker.Close()
	if writeErr != nil || written != len("autopus-owned\n") || closeErr != nil {
		_ = root.Remove(workflowContextManagedRuntimeMarker)
		if writeErr == nil && written != len("autopus-owned\n") {
			writeErr = errors.New("managed OMP runtime lease write was incomplete")
		}
		return errors.Join(writeErr, closeErr)
	}
	return nil
}

func openWorkflowContextManagedRuntime(
	options WorkflowContextManagedRPCOptions,
) (*os.Root, *os.Root, fs.FileInfo, error) {
	baseInfo, err := os.Lstat(options.RuntimeBase)
	if err != nil || !baseInfo.IsDir() || baseInfo.Mode()&os.ModeSymlink != 0 {
		return nil, nil, nil, errors.New("managed OMP runtime base identity is invalid")
	}
	baseRoot, err := os.OpenRoot(options.RuntimeBase)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open managed OMP runtime base: %w", err)
	}
	info, err := baseRoot.Lstat("omp-runtime")
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		_ = baseRoot.Close()
		return nil, nil, nil, errors.New("managed OMP runtime root identity is invalid")
	}
	directoryRoot, err := baseRoot.OpenRoot("omp-runtime")
	if err != nil || !sameWorkflowContextManagedRuntime(directoryRoot, info) {
		if directoryRoot != nil {
			_ = directoryRoot.Close()
		}
		_ = baseRoot.Close()
		return nil, nil, nil, errors.New("managed OMP runtime root changed while opening")
	}
	return baseRoot, directoryRoot, info, nil
}

func sameWorkflowContextManagedRuntime(root *os.Root, expected fs.FileInfo) bool {
	if root == nil || expected == nil {
		return false
	}
	current, err := root.Stat(".")
	return err == nil && current.IsDir() && os.SameFile(current, expected)
}

func validateInitialWorkflowContextManagedRuntime(
	root *os.Root, options WorkflowContextManagedRPCOptions, leased bool,
) error {
	allowedDirs := map[string]struct{}{".": {}}
	allowedFiles := make(map[string]struct{})
	addPath := func(path string, directory bool) error {
		relative, err := filepath.Rel(options.RuntimeRoot, path)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errors.New("managed OMP initial artifact is outside the runtime root")
		}
		relative = filepath.ToSlash(relative)
		for parent := filepath.ToSlash(filepath.Dir(relative)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			allowedDirs[parent] = struct{}{}
		}
		if directory {
			allowedDirs[relative] = struct{}{}
		} else {
			allowedFiles[relative] = struct{}{}
		}
		return nil
	}
	if err := addPath(options.SessionDir, true); err != nil {
		return err
	}
	if err := addPath(options.ConfigPath, false); err != nil {
		return err
	}
	models := filepath.Join(options.RuntimeRoot, "models.yml")
	if _, err := os.Lstat(models); err == nil {
		if err := addPath(models, false); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect managed OMP model config: %w", err)
	}
	for _, entry := range options.Environment {
		_, value, found := strings.Cut(entry, "=")
		if !found || !filepath.IsAbs(value) {
			continue
		}
		canonical, err := canonicalWorkflowContextManagedPath(value, false)
		if err != nil || !workflowContextManagedPathBelow(options.RuntimeRoot, canonical) {
			continue
		}
		info, err := os.Lstat(canonical)
		if err != nil || !info.IsDir() {
			continue
		}
		if err := addPath(canonical, true); err != nil {
			return err
		}
	}
	if leased {
		allowedFiles[workflowContextManagedRuntimeMarker] = struct{}{}
	}
	return fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("managed OMP initial runtime contains a symlink")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if _, ok := allowedDirs[path]; !ok || info.Mode().Perm()&0o077 != 0 {
				return fmt.Errorf("managed OMP initial runtime contains an unowned directory: %s", path)
			}
			return nil
		}
		if _, ok := allowedFiles[path]; !ok || !info.Mode().IsRegular() || info.Mode().Perm()&^fs.FileMode(0o600) != 0 {
			if path == workflowContextManagedRuntimeMarker && !leased {
				return errors.New("acquire exclusive runtime lease: marker already exists")
			}
			return fmt.Errorf("managed OMP initial runtime contains an unowned artifact: %s", path)
		}
		return nil
	})
}

// @AX:WARN [AUTO]: managed runtime deletion has 15 manual cyclomatic decision points.
// @AX:REASON [AUTO]: root identity, symlink rejection, lease ownership, directory contents, and post-delete readback must all fail closed.
func removeWorkflowContextManagedRuntime(
	baseRoot, directoryRoot *os.Root, expected fs.FileInfo,
) bool {
	if baseRoot == nil || !sameWorkflowContextManagedRuntime(directoryRoot, expected) {
		return false
	}
	current, err := baseRoot.Lstat("omp-runtime")
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(current, expected) {
		return false
	}
	marker, err := directoryRoot.ReadFile(workflowContextManagedRuntimeMarker)
	if err != nil || string(marker) != "autopus-owned\n" {
		return false
	}
	entries, err := fs.ReadDir(directoryRoot.FS(), ".")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if err := directoryRoot.RemoveAll(entry.Name()); err != nil {
			return false
		}
	}
	current, err = baseRoot.Lstat("omp-runtime")
	if err != nil || !os.SameFile(current, expected) || !sameWorkflowContextManagedRuntime(directoryRoot, current) {
		return false
	}
	if err := baseRoot.Remove("omp-runtime"); err != nil {
		return false
	}
	_, err = baseRoot.Lstat("omp-runtime")
	return errors.Is(err, fs.ErrNotExist)
}
