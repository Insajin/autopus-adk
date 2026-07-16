package design

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const (
	visualReportDir    = ".autopus/design/verify"
	visualReportV1Path = visualReportDir + "/latest.json"
	visualReportV2Path = visualReportDir + "/latest.v2.json"
)

type visualReportRoot interface {
	Lstat(name string) (os.FileInfo, error)
	Mkdir(name string, perm os.FileMode) error
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	Remove(name string) error
	Rename(oldname, newname string) error
}

func openVisualReportRoot(path string) (*os.Root, string, error) {
	rootAbs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, "", err
	}
	info, err := os.Lstat(resolvedRoot)
	if err != nil {
		return nil, "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, "", fmt.Errorf("visual report root must resolve to a directory")
	}
	root, err := os.OpenRoot(resolvedRoot)
	if err != nil {
		return nil, "", err
	}
	if err := validateOpenedVisualRoot(info, root); err != nil {
		_ = root.Close()
		return nil, "", err
	}
	return root, rootAbs, nil
}

func openVisualReportRootForWrite(path string) (*os.Root, string, error) {
	rootAbs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(rootAbs, 0o755); err != nil {
		return nil, "", err
	}
	return openVisualReportRoot(rootAbs)
}

func validateOpenedVisualRoot(expected os.FileInfo, root *os.Root) error {
	actual, err := root.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(expected, actual) {
		return fmt.Errorf("visual report root changed while opening")
	}
	return nil
}

func ensureVisualReportDir(root visualReportRoot) error {
	for _, component := range []string{".autopus", ".autopus/design", visualReportDir} {
		component = filepath.FromSlash(component)
		info, err := root.Lstat(component)
		if os.IsNotExist(err) {
			if err := root.Mkdir(component, 0o755); err != nil && !os.IsExist(err) {
				return err
			}
			info, err = root.Lstat(component)
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("visual report path contains symlink: %s", component)
		}
		if !info.IsDir() {
			return fmt.Errorf("visual report path component is not a directory: %s", component)
		}
	}
	return nil
}

func createVisualReportTemp(root visualReportRoot) (*os.File, string, error) {
	for range 8 {
		suffix := make([]byte, 12)
		if _, err := rand.Read(suffix); err != nil {
			return nil, "", err
		}
		path := filepath.Join(filepath.FromSlash(visualReportDir), ".visual-report-"+hex.EncodeToString(suffix))
		file, err := root.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			return file, path, nil
		}
		if !os.IsExist(err) {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("could not allocate visual report temp file")
}

func validateVisualReportTarget(root visualReportRoot, path string) error {
	info, err := root.Lstat(filepath.FromSlash(path))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("visual report target is a symlink")
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("visual report target is not a regular file")
	}
	return nil
}
