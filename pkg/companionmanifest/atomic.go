package companionmanifest

import (
	"errors"
	"os"
	"path/filepath"
)

// WriteAtomic replaces a regular output with complete 0600 bytes in one directory.
func WriteAtomic(path string, data []byte) (returnErr error) {
	cleanPath := filepath.Clean(path)
	if cleanPath == "." || filepath.Base(cleanPath) == "." {
		return errors.New("invalid output path")
	}
	dir := filepath.Dir(cleanPath)
	dirInfo, err := os.Lstat(dir)
	if err != nil || !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("output directory must be a regular directory")
	}
	if info, statErr := os.Lstat(cleanPath); statErr == nil {
		if !info.Mode().IsRegular() {
			return errors.New("output must be a regular file")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return errors.New("inspect output path")
	}

	temp, err := os.CreateTemp(dir, "."+filepath.Base(cleanPath)+".tmp-")
	if err != nil {
		return errors.New("create atomic output")
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		if removeErr := os.Remove(tempPath); returnErr == nil && removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			returnErr = errors.New("remove atomic temporary output")
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		return errors.New("secure atomic output")
	}
	if err := writeAll(temp, data); err != nil {
		return errors.New("write atomic output")
	}
	if err := temp.Sync(); err != nil {
		return errors.New("sync atomic output")
	}
	if err := temp.Close(); err != nil {
		return errors.New("close atomic output")
	}
	if err := os.Rename(tempPath, cleanPath); err != nil {
		return errors.New("commit atomic output")
	}
	return syncDirectory(dir)
}

// WriteSignedFiles transactionally replaces a manifest and detached signature pair.
func WriteSignedFiles(manifestPath, signaturePath string, manifest, signature []byte) error {
	return writeSignedFilesWithFault(
		manifestPath,
		signaturePath,
		manifest,
		signature,
		nil,
	)
}

func writeAll(file *os.File, data []byte) error {
	for len(data) > 0 {
		written, err := file.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return errors.New("short write")
		}
		data = data[written:]
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return errors.New("open output directory")
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return errors.New("sync output directory")
	}
	return nil
}
