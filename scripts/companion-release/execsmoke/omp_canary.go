package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func validatePinnedOMPExecutable(path string, policy ompCanaryPolicy) (string, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", errors.New("arm64 OMP executable must be an absolute canonical path")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect arm64 OMP executable: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 ||
		info.Mode().Perm()&0o022 != 0 {
		return "", errors.New("arm64 OMP executable must be a non-symlink non-writable regular executable")
	}
	if err := validateMachOArchitecture(path, "arm64"); err != nil {
		return "", fmt.Errorf("validate arm64 OMP executable: %w", err)
	}
	digest, err := stableExecutableSHA256(path, info)
	if err != nil {
		return "", err
	}
	if policy.version != pinnedOMPVersion {
		return "", errors.New("pinned OMP version is unavailable")
	}
	if policy.sha256 == "" || digest != policy.sha256 {
		return "", errors.New("arm64 OMP executable SHA-256 mismatch")
	}
	return path, nil
}

func stableExecutableSHA256(path string, before os.FileInfo) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open arm64 OMP executable: %w", err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	after, statErr := file.Stat()
	closeErr := file.Close()
	if copyErr != nil || statErr != nil || closeErr != nil || !os.SameFile(before, after) ||
		before.Size() != after.Size() || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		return "", errors.New("arm64 OMP executable changed while hashing")
	}
	return "sha256:" + fmtDigest(hash.Sum(nil)), nil
}

func fmtDigest(value []byte) string {
	const digits = "0123456789abcdef"
	result := make([]byte, len(value)*2)
	for index, item := range value {
		result[index*2], result[index*2+1] = digits[item>>4], digits[item&0xf]
	}
	return string(result)
}
