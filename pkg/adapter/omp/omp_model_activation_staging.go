package omp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func stageOMPModelActivationConfig(config []byte) (string, func() error, error) {
	directory, err := os.MkdirTemp("", "autopus-omp-activation-")
	if err != nil {
		return "", nil, fmt.Errorf("create activation staging root: %w", err)
	}
	cleanup := func() error { return os.RemoveAll(directory) }
	if err := os.Chmod(directory, 0o700); err != nil {
		return "", nil, errors.Join(fmt.Errorf("secure activation staging root: %w", err), cleanup())
	}
	path := filepath.Join(directory, "config.yml")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", nil, errors.Join(fmt.Errorf("create activation config snapshot: %w", err), cleanup())
	}
	written, writeErr := file.Write(config)
	if writeErr == nil && written != len(config) {
		writeErr = errors.New("activation config snapshot short write")
	}
	writeErr = errors.Join(writeErr, file.Sync(), file.Close())
	if writeErr != nil {
		return "", nil, errors.Join(fmt.Errorf("write activation config snapshot: %w", writeErr), cleanup())
	}
	return path, cleanup, nil
}

func replaceOMPModelConfigArg(args []string, stagedPath string) []string {
	replaced := append([]string(nil), args...)
	for i := 0; i+1 < len(replaced); i++ {
		if replaced[i] == "--config" {
			replaced[i+1] = stagedPath
			break
		}
	}
	return replaced
}
