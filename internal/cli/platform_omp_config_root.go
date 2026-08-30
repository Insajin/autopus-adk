package cli

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	autopusConfigName         = "autopus.yaml"
	maxAutopusConfigFileBytes = 1 << 20
)

type autopusConfigSnapshot struct {
	root       *os.Root
	rootPath   string
	rootInfo   fs.FileInfo
	configInfo fs.FileInfo
	mode       os.FileMode
	data       []byte
}

func openAutopusConfigSnapshot(path string) (*autopusConfigSnapshot, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.New("workspace_root_unsafe")
	}
	inputInfo, err := os.Lstat(absolute)
	if err != nil || !inputInfo.IsDir() || inputInfo.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("workspace_root_unsafe")
	}
	realPath, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, errors.New("workspace_root_unsafe")
	}
	realInfo, err := os.Lstat(realPath)
	inputAfter, inputErr := os.Lstat(absolute)
	if err != nil || inputErr != nil || !realInfo.IsDir() || realInfo.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(inputInfo, inputAfter) || !os.SameFile(inputInfo, realInfo) {
		return nil, errors.New("workspace_root_unsafe")
	}
	root, err := os.OpenRoot(realPath)
	if err != nil {
		return nil, errors.New("workspace_root_unsafe")
	}
	snapshot := &autopusConfigSnapshot{root: root, rootPath: realPath, rootInfo: realInfo}
	if !snapshot.sameRoot() {
		_ = root.Close()
		return nil, errors.New("workspace_root_unsafe")
	}
	info, err := root.Lstat(autopusConfigName)
	if errors.Is(err, fs.ErrNotExist) {
		_ = root.Close()
		return nil, errors.New("autopus_config_missing")
	}
	if err != nil {
		_ = root.Close()
		return nil, errors.New("autopus_config_unreadable")
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		_ = root.Close()
		return nil, errors.New("autopus_config_unsafe")
	}
	data, boundInfo, err := snapshot.readBoundConfig(info, "autopus_config_unreadable")
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	snapshot.configInfo = boundInfo
	snapshot.mode = boundInfo.Mode().Perm()
	snapshot.data = data
	return snapshot, nil
}

func (snapshot *autopusConfigSnapshot) Close() {
	if snapshot != nil && snapshot.root != nil {
		_ = snapshot.root.Close()
	}
}

func (snapshot *autopusConfigSnapshot) Verify() error {
	data, info, err := snapshot.readBoundConfig(snapshot.configInfo, "autopus_config_changed")
	if err != nil || info.Mode().Perm() != snapshot.mode || !bytes.Equal(data, snapshot.data) {
		return errors.New("autopus_config_changed")
	}
	return nil
}

func (snapshot *autopusConfigSnapshot) Replace(data []byte) error {
	if err := snapshot.Verify(); err != nil {
		return err
	}
	tempName, temp, err := createAutopusConfigTemp(snapshot.root)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = snapshot.root.Remove(tempName)
		}
	}()
	if err := temp.Chmod(snapshot.mode); err != nil {
		return err
	}
	if _, err := io.Copy(temp, bytes.NewReader(data)); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	newInfo, err := temp.Stat()
	if err != nil || !newInfo.Mode().IsRegular() || newInfo.Mode().Perm() != snapshot.mode {
		return errors.New("autopus_config_write_failed")
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := snapshot.Verify(); err != nil {
		return err
	}
	if err := snapshot.root.Rename(tempName, autopusConfigName); err != nil {
		return err
	}
	committed = true
	snapshot.configInfo = newInfo
	snapshot.data = bytes.Clone(data)
	current, err := snapshot.root.Lstat(autopusConfigName)
	if err != nil || !current.Mode().IsRegular() || current.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(current, newInfo) {
		return errors.New("autopus_config_changed")
	}
	snapshot.configInfo = current
	if err := snapshot.syncRoot(); err != nil {
		return err
	}
	return snapshot.Verify()
}

func (snapshot *autopusConfigSnapshot) sameRoot() bool {
	if snapshot == nil || snapshot.root == nil || snapshot.rootInfo == nil {
		return false
	}
	handleInfo, handleErr := snapshot.root.Stat(".")
	pathInfo, pathErr := os.Lstat(snapshot.rootPath)
	return handleErr == nil && pathErr == nil && handleInfo.IsDir() && pathInfo.IsDir() &&
		pathInfo.Mode()&os.ModeSymlink == 0 && os.SameFile(handleInfo, snapshot.rootInfo) &&
		os.SameFile(pathInfo, snapshot.rootInfo)
}

func (snapshot *autopusConfigSnapshot) readBoundConfig(
	expected fs.FileInfo,
	failure string,
) ([]byte, fs.FileInfo, error) {
	if !snapshot.sameRoot() {
		return nil, nil, errors.New(failure)
	}
	info, err := snapshot.root.Lstat(autopusConfigName)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		expected == nil || !os.SameFile(info, expected) || info.Mode().Perm() != expected.Mode().Perm() {
		return nil, nil, errors.New(failure)
	}
	file, err := snapshot.root.Open(autopusConfigName)
	if err != nil {
		return nil, nil, errors.New(failure)
	}
	opened, statErr := file.Stat()
	if statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(opened, info) {
		_ = file.Close()
		return nil, nil, errors.New(failure)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxAutopusConfigFileBytes+1))
	closeErr := file.Close()
	current, currentErr := snapshot.root.Lstat(autopusConfigName)
	if readErr != nil || closeErr != nil || len(data) > maxAutopusConfigFileBytes ||
		currentErr != nil || !current.Mode().IsRegular() || current.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(current, opened) || current.Mode().Perm() != info.Mode().Perm() ||
		int64(len(data)) != opened.Size() || !snapshot.sameRoot() {
		return nil, nil, errors.New(failure)
	}
	return data, current, nil
}

func (snapshot *autopusConfigSnapshot) syncRoot() error {
	directory, err := snapshot.root.Open(".")
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	return errors.Join(syncErr, closeErr)
}

func createAutopusConfigTemp(root *os.Root) (string, *os.File, error) {
	for attempt := 0; attempt < 16; attempt++ {
		random := make([]byte, 8)
		if _, err := rand.Read(random); err != nil {
			return "", nil, err
		}
		name := ".autopus.yaml.tmp-" + hex.EncodeToString(random)
		file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		return name, file, nil
	}
	return "", nil, errors.New("autopus_config_temp_exhausted")
}
