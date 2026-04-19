package arch

import (
	"os"
	"path/filepath"
	"strings"
)

// detectProjectType는 프로젝트 유형을 감지한다.
func detectProjectType(dir string) string {
	if fileExists(filepath.Join(dir, "go.mod")) {
		return "go"
	}
	if fileExists(filepath.Join(dir, "package.json")) {
		return "ts"
	}
	if fileExists(filepath.Join(dir, "setup.py")) ||
		fileExists(filepath.Join(dir, "pyproject.toml")) ||
		fileExists(filepath.Join(dir, "requirements.txt")) {
		return "python"
	}
	return "unknown"
}

func relativePackage(base, path string) string {
	rel, err := filepath.Rel(base, filepath.Dir(path))
	if err != nil {
		return path
	}
	return rel
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func extractGoModule(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
