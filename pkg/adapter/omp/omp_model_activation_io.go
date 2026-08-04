package omp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func OMPModelSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validOMPModelHash(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

// @AX:WARN [AUTO]: owned-path resolution has cyclomatic complexity 20.
// @AX:REASON [AUTO]: gocyclo reports 20 across canonical-root, traversal, symlink, directory creation, and containment checks.
func resolveOMPModelOwnedPath(root, relative string, createParents bool) (string, error) {
	if root == "" || relative == "" || filepath.IsAbs(relative) {
		return "", fmt.Errorf("workspace root and relative owned path are required")
	}
	cleanRelative := filepath.Clean(relative)
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("owned path escapes workspace")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	rootInfo, err := os.Lstat(absRoot)
	if err != nil {
		return "", fmt.Errorf("inspect workspace root: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return "", fmt.Errorf("workspace root must be a non-symlink directory")
	}
	canonicalRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("canonicalize workspace root: %w", err)
	}
	target := filepath.Join(absRoot, cleanRelative)
	if !pathInsideOMPModelRoot(absRoot, target) {
		return "", fmt.Errorf("owned path escapes workspace")
	}
	parent := filepath.Dir(target)
	if err := ensureOMPModelParents(absRoot, parent, createParents); err != nil {
		return "", err
	}
	canonicalParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", fmt.Errorf("canonicalize owned parent: %w", err)
	}
	if !pathInsideOMPModelRoot(canonicalRoot, canonicalParent) {
		return "", fmt.Errorf("owned parent escapes workspace")
	}
	if info, statErr := os.Lstat(target); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", fmt.Errorf("owned path must be a regular non-symlink file")
		}
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("inspect owned path: %w", statErr)
	}
	return target, nil
}

func ensureOMPModelParents(root, parent string, create bool) error {
	relative, err := filepath.Rel(root, parent)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("owned parent escapes workspace")
	}
	current := root
	if relative == "." {
		return nil
	}
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) && create {
			if mkdirErr := os.Mkdir(current, 0o700); mkdirErr != nil && !os.IsExist(mkdirErr) {
				return fmt.Errorf("create owned parent: %w", mkdirErr)
			}
			info, statErr = os.Lstat(current)
		}
		if statErr != nil {
			return fmt.Errorf("inspect owned parent: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("owned parent must be a non-symlink directory")
		}
	}
	return nil
}

func pathInsideOMPModelRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// @AX:WARN [AUTO]: owner-only atomic model write contains 8 if branches.
// @AX:REASON [AUTO]: parent validation, exclusive temp creation, permissions, write, sync, close, and rename must all succeed.
func readOMPModelOwnedFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open owned file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("owned file must be regular")
	}
	data, err := io.ReadAll(io.LimitReader(file, (4<<20)+1))
	if err != nil {
		return nil, fmt.Errorf("read owned file: %w", err)
	}
	if len(data) > 4<<20 {
		return nil, fmt.Errorf("owned file exceeds size limit")
	}
	return data, nil
}

func readOMPModelOwnedFileAt(workspace *ompRootedWorkspace, relative string) ([]byte, error) {
	file, _, err := workspace.openRegular(relative)
	if err != nil {
		return nil, fmt.Errorf("open owned file: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (4<<20)+1))
	if err != nil {
		return nil, fmt.Errorf("read owned file: %w", err)
	}
	if len(data) > 4<<20 {
		return nil, fmt.Errorf("owned file exceeds size limit")
	}
	return data, nil
}

func validateOMPModelWorkspaceBinding(workspace *ompRootedWorkspace) error {
	current, err := os.Lstat(workspace.path)
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !current.IsDir() ||
		!os.SameFile(current, workspace.info) {
		return fmt.Errorf("OMP workspace changed during activation verification")
	}
	opened, err := workspace.root.Stat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(opened, workspace.info) {
		return fmt.Errorf("OMP workspace changed during activation verification")
	}
	return nil
}

func requireExactOMPModelConfigArg(args []string, expected string) error {
	found := 0
	for i := 0; i < len(args); i++ {
		if args[i] != "--config" {
			continue
		}
		if i+1 >= len(args) || args[i+1] != expected {
			return fmt.Errorf("--config must reference the exact overlay")
		}
		found++
		i++
	}
	if found != 1 {
		return fmt.Errorf("exactly one --config overlay argument is required")
	}
	return nil
}
