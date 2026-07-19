package codex

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func writeCodexManagedFile(rootPath, relativePath string, data []byte, mode os.FileMode) (returnErr error) {
	clean, err := cleanCodexManagedPath(relativePath)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return fmt.Errorf("open repository root: %w", err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close repository root: %w", err))
		}
	}()

	if err := ensureCodexManagedDirs(root, filepath.Dir(clean), true); err != nil {
		return err
	}
	if info, statErr := root.Lstat(clean); statErr == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("managed target must be a regular file: %s", clean)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect managed target %s: %w", clean, statErr)
	}

	tempName, file, err := createCodexManagedTemp(root, filepath.Dir(clean), filepath.Base(clean))
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = root.Remove(tempName)
		}
	}()

	if _, err := io.Copy(file, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("write managed temporary file: %w", err)
	}
	if err := file.Chmod(mode.Perm()); err != nil {
		return fmt.Errorf("set managed file mode: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync managed file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close managed file: %w", err)
	}
	if err := root.Rename(tempName, clean); err != nil {
		return fmt.Errorf("commit managed file %s: %w", clean, err)
	}
	committed = true
	return nil
}

func removeCodexHookAssets(rootPath string) (returnErr error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return fmt.Errorf("open repository root: %w", err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close repository root: %w", err))
		}
	}()

	assetDir := filepath.Join(".codex", "hooks", "autopus")
	if err := ensureCodexManagedDirs(root, assetDir, false); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, name := range codexHookAssetNames {
		path := filepath.Join(assetDir, name)
		if err := root.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove managed Codex hook %s: %w", path, err)
		}
	}
	// Keep directories that contain user-owned hooks; remove only empty shells.
	_ = root.Remove(assetDir)
	_ = root.Remove(filepath.Join(".codex", "hooks"))
	return nil
}

func cleanCodexManagedPath(path string) (string, error) {
	clean := filepath.Clean(path)
	if clean == "." || filepath.IsAbs(clean) || !filepath.IsLocal(clean) {
		return "", fmt.Errorf("invalid managed path: %s", path)
	}
	return clean, nil
}

func ensureCodexManagedDirs(root *os.Root, dir string, create bool) error {
	if dir == "." {
		return nil
	}
	current := ""
	for _, part := range strings.Split(filepath.Clean(dir), string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := root.Lstat(current)
		if errors.Is(err, os.ErrNotExist) && create {
			if err := root.Mkdir(current, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("create managed directory %s: %w", current, err)
			}
			info, err = root.Lstat(current)
		}
		if err != nil {
			return fmt.Errorf("inspect managed directory %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("managed directory must not be a symlink: %s", current)
		}
	}
	return nil
}

func createCodexManagedTemp(root *os.Root, dir, base string) (string, *os.File, error) {
	for attempt := 0; attempt < 16; attempt++ {
		random := make([]byte, 8)
		if _, err := rand.Read(random); err != nil {
			return "", nil, fmt.Errorf("generate managed temporary name: %w", err)
		}
		name := "." + base + ".tmp-" + hex.EncodeToString(random)
		path := filepath.Join(dir, name)
		file, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", nil, fmt.Errorf("create managed temporary file: %w", err)
		}
		return path, file, nil
	}
	return "", nil, errors.New("exhausted managed temporary file names")
}
