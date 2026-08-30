package omp

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type ompRootedWorkspace struct {
	root *os.Root
	path string
	info fs.FileInfo
}

func joinOMPRootedCloseError(returnErr *error, closeErr error) {
	if closeErr != nil {
		*returnErr = errors.Join(*returnErr, fmt.Errorf("close rooted OMP handle: %w", closeErr))
	}
}

func openOMPRootedWorkspace(path string) (*ompRootedWorkspace, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve OMP workspace: %w", err)
	}
	before, err := os.Lstat(abs)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("OMP workspace must be a real directory")
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("canonicalize OMP workspace: %w", err)
	}
	root, err := os.OpenRoot(canonical)
	if err != nil {
		return nil, fmt.Errorf("open OMP workspace: %w", err)
	}
	after, statErr := os.Lstat(abs)
	opened, openErr := root.Stat(".")
	if statErr != nil || openErr != nil || after.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(before, after) || !os.SameFile(before, opened) {
		_ = root.Close()
		return nil, fmt.Errorf("OMP workspace changed while opening")
	}
	return &ompRootedWorkspace{root: root, path: abs, info: opened}, nil
}

func (workspace *ompRootedWorkspace) Close() error {
	if workspace == nil || workspace.root == nil {
		return nil
	}
	cleanupErr := workspace.cleanupOMPConfigRollbackArtifactsIfUnowned()
	return errors.Join(cleanupErr, workspace.root.Close())
}

func cleanOMPRootedPath(path string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) ||
		(len(clean) >= 2 && clean[1] == ':') {
		return "", fmt.Errorf("unsafe OMP workspace path %s", path)
	}
	return clean, nil
}

func (workspace *ompRootedWorkspace) absolute(path string) (string, error) {
	clean, err := cleanOMPRootedPath(path)
	if err != nil {
		return "", err
	}
	return filepath.Join(workspace.path, clean), nil
}

func (workspace *ompRootedWorkspace) openDir(path string, create bool, perm os.FileMode) (*os.Root, error) {
	current, err := workspace.root.OpenRoot(".")
	if err != nil || !sameOMPRootedDirectory(current, workspace.info) {
		if current != nil {
			_ = current.Close()
		}
		return nil, fmt.Errorf("open bound OMP workspace")
	}
	if path == "." || path == "" {
		return current, nil
	}
	clean, err := cleanOMPRootedPath(filepath.Join(path, ".guard"))
	if err != nil {
		_ = current.Close()
		return nil, err
	}
	clean = filepath.Dir(clean)
	for _, component := range strings.Split(clean, string(filepath.Separator)) {
		next, openErr := openOMPRootedChild(current, component, create, perm)
		_ = current.Close()
		if openErr != nil {
			return nil, openErr
		}
		current = next
	}
	return current, nil
}

func openOMPRootedChild(parent *os.Root, name string, create bool, perm os.FileMode) (*os.Root, error) {
	before, err := parent.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) && create {
		if err = parent.Mkdir(name, perm); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("create OMP directory %s: %w", name, err)
		}
		before, err = parent.Lstat(name)
	}
	if err != nil {
		return nil, fmt.Errorf("inspect OMP path component %s: %w", name, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("OMP path component %s is not a real directory", name)
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("open OMP directory %s: %w", name, err)
	}
	after, statErr := parent.Lstat(name)
	if statErr != nil || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, after) ||
		!sameOMPRootedDirectory(child, after) {
		_ = child.Close()
		return nil, fmt.Errorf("OMP path component %s changed while opening", name)
	}
	return child, nil
}

func sameOMPRootedDirectory(root *os.Root, expected fs.FileInfo) bool {
	if root == nil || expected == nil {
		return false
	}
	opened, err := root.Stat(".")
	return err == nil && opened.IsDir() && os.SameFile(expected, opened)
}

func (workspace *ompRootedWorkspace) openParent(path string, create bool) (*os.Root, string, error) {
	clean, err := cleanOMPRootedPath(path)
	if err != nil {
		return nil, "", err
	}
	parent, err := workspace.openDir(filepath.Dir(clean), create, 0o755)
	return parent, filepath.Base(clean), err
}

func (workspace *ompRootedWorkspace) lstat(path string) (info fs.FileInfo, returnErr error) {
	parent, name, err := workspace.openParent(path, false)
	if err != nil {
		return nil, err
	}
	defer func() { joinOMPRootedCloseError(&returnErr, parent.Close()) }()
	return parent.Lstat(name)
}

func (workspace *ompRootedWorkspace) openRegular(path string) (file *os.File, info fs.FileInfo, returnErr error) {
	parent, name, err := workspace.openParent(path, false)
	if err != nil {
		return nil, nil, err
	}
	defer func() { joinOMPRootedCloseError(&returnErr, parent.Close()) }()
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, nil, fmt.Errorf("inspect OMP path %s: %w", path, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("OMP path %s is not a regular file", path)
	}
	file, err = parent.Open(name)
	if err != nil {
		return nil, nil, err
	}
	opened, openErr := file.Stat()
	after, statErr := parent.Lstat(name)
	if openErr != nil || statErr != nil || after.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(before, opened) || !os.SameFile(opened, after) {
		_ = file.Close()
		return nil, nil, fmt.Errorf("OMP path %s changed while opening", path)
	}
	return file, opened, nil
}

func (workspace *ompRootedWorkspace) readFile(path string, limit int64) ([]byte, fs.FileInfo, error) {
	file, info, err := workspace.openRegular(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	reader := io.Reader(file)
	if limit > 0 {
		reader = io.LimitReader(file, limit+1)
	}
	data, err := io.ReadAll(reader)
	if err != nil || limit > 0 && int64(len(data)) > limit {
		return nil, nil, fmt.Errorf("read OMP path %s: size or IO failure", path)
	}
	after, statErr := workspace.lstat(path)
	if statErr != nil || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, after) {
		return nil, nil, fmt.Errorf("read OMP path %s: identity changed", path)
	}
	return data, info, nil
}

func (workspace *ompRootedWorkspace) readOwnerOnlyFile(path string, limit int64) ([]byte, fs.FileInfo, error) {
	data, info, err := workspace.readFile(path, limit)
	if err != nil {
		return nil, nil, err
	}
	if info.Mode().Perm() != 0o600 {
		return nil, nil, fmt.Errorf("OMP path %s must have mode 0600", path)
	}
	return data, info, nil
}

func (workspace *ompRootedWorkspace) atomicWrite(path string, data []byte, perm os.FileMode) (returnErr error) {
	parent, name, err := workspace.openParent(path, true)
	if err != nil {
		return err
	}
	defer func() { joinOMPRootedCloseError(&returnErr, parent.Close()) }()
	if info, statErr := parent.Lstat(name); statErr == nil &&
		(info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return fmt.Errorf("OMP write target %s is not a regular file", path)
	} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return statErr
	}
	tempName, file, err := createOMPRootedTemp(parent, perm)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
		if returnErr != nil {
			_ = parent.Remove(tempName)
		}
	}()
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Chmod(perm); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return parent.Rename(tempName, name)
}

func createOMPRootedTemp(parent *os.Root, perm os.FileMode) (string, *os.File, error) {
	for range 100 {
		var entropy [12]byte
		if _, err := rand.Read(entropy[:]); err != nil {
			return "", nil, err
		}
		name := ".autopus-omp-" + hex.EncodeToString(entropy[:])
		file, err := parent.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return name, file, err
	}
	return "", nil, fmt.Errorf("OMP temporary name exhausted")
}

func (workspace *ompRootedWorkspace) remove(path string, recursive bool) (returnErr error) {
	parent, name, err := workspace.openParent(path, false)
	if err != nil {
		return err
	}
	defer func() { joinOMPRootedCloseError(&returnErr, parent.Close()) }()
	if recursive {
		return parent.RemoveAll(name)
	}
	return parent.Remove(name)
}

func (workspace *ompRootedWorkspace) readDir(path string) (entries []os.DirEntry, returnErr error) {
	root, err := workspace.openDir(path, false, 0)
	if err != nil {
		return nil, err
	}
	defer func() { joinOMPRootedCloseError(&returnErr, root.Close()) }()
	directory, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	return directory.ReadDir(-1)
}
