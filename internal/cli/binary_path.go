package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

type binaryPathInfo struct {
	ExecutablePath string
	ResolvedPath   string
}

var (
	currentExecutablePath = os.Executable
	evalBinarySymlinks    = filepath.EvalSymlinks
)

func resolveCurrentBinaryPath() (binaryPathInfo, error) {
	execPath, err := currentExecutablePath()
	if err != nil {
		return binaryPathInfo{}, fmt.Errorf("현재 바이너리 경로를 가져올 수 없음: %w", err)
	}

	resolvedPath, err := evalBinarySymlinks(execPath)
	if err != nil {
		resolvedPath = execPath
	}

	return binaryPathInfo{
		ExecutablePath: execPath,
		ResolvedPath:   resolvedPath,
	}, nil
}

func (info binaryPathInfo) ManagedPath() string {
	if info.ResolvedPath != "" {
		return info.ResolvedPath
	}
	return info.ExecutablePath
}

func (info binaryPathInfo) IsSymlinked() bool {
	return info.ExecutablePath != "" && info.ExecutablePath != info.ManagedPath()
}
