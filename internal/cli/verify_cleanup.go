package cli

import (
	"errors"
	"fmt"
)

func cleanupPlaywrightTempDir(path string, runErr error, removeAll func(string) error) error {
	cleanupErr := removeAll(path)
	if cleanupErr == nil {
		return runErr
	}
	cleanupErr = fmt.Errorf("playwright 임시 디렉터리 정리 실패: %w", cleanupErr)
	if runErr == nil {
		return cleanupErr
	}
	return errors.Join(runErr, cleanupErr)
}
