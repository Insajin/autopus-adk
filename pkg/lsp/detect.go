package lsp

import (
	"fmt"
	"os"
	"path/filepath"
)

// DetectServer는 프로젝트 디렉터리에서 적합한 LSP 서버를 자동 감지한다.
func DetectServer(projectDir string) (serverCmd string, args []string, err error) {
	if _, err := os.Stat(projectDir); err != nil {
		return "", nil, fmt.Errorf("프로젝트 디렉터리 접근 실패: %w", err)
	}

	// Go 프로젝트 감지
	if fileExists(filepath.Join(projectDir, "go.mod")) {
		return "gopls", []string{}, nil
	}

	// TypeScript/JavaScript 프로젝트 감지
	if fileExists(filepath.Join(projectDir, "package.json")) {
		return "typescript-language-server", []string{"--stdio"}, nil
	}

	// Python 프로젝트 감지
	if fileExists(filepath.Join(projectDir, "setup.py")) ||
		fileExists(filepath.Join(projectDir, "pyproject.toml")) ||
		fileExists(filepath.Join(projectDir, "requirements.txt")) {
		return "pyright", []string{"--stdio"}, nil
	}

	return "", nil, fmt.Errorf("알 수 없는 프로젝트 유형: LSP 서버를 감지할 수 없습니다")
}

// fileExists는 파일 또는 디렉터리의 존재 여부를 확인한다.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
