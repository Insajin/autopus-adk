package omp

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

func writeMappings(root string, files []adapter.FileMapping) error {
	for _, file := range files {
		if err := writeMapping(root, file); err != nil {
			return err
		}
	}
	return nil
}

func writeMapping(root string, file adapter.FileMapping) error {
	targetPath, err := adapter.SafeWorkspacePath(root, file.TargetPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("디렉터리 생성 실패 %s: %w", filepath.Dir(targetPath), err)
	}
	perm := os.FileMode(0o644)
	if isOwnerOnlyOMPModelPath(file.TargetPath) {
		perm = ompConfigFileMode
		if filepath.ToSlash(file.TargetPath) == configFile {
			if info, statErr := os.Stat(targetPath); statErr == nil {
				perm = info.Mode().Perm()
			}
		}
	}
	if err := os.WriteFile(targetPath, file.Content, perm); err != nil {
		return fmt.Errorf("파일 쓰기 실패 %s: %w", file.TargetPath, err)
	}
	return nil
}
