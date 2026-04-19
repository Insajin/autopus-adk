package lsp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/lsp"
)

func writeProjectFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

// TestDetectServer_PyprojectToml은 pyproject.toml 기반 Python 프로젝트 감지를 테스트한다.
func TestDetectServer_PyprojectToml(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeProjectFile(t, dir, "pyproject.toml", "[build-system]\n")

	serverCmd, args, err := lsp.DetectServer(dir)
	require.NoError(t, err)
	assert.Equal(t, "pyright", serverCmd)
	assert.Contains(t, args, "--stdio")
}

// TestDetectServer_RequirementsTxt는 requirements.txt 기반 Python 프로젝트 감지를 테스트한다.
func TestDetectServer_RequirementsTxt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeProjectFile(t, dir, "requirements.txt", "fastapi\npydantic\n")

	serverCmd, args, err := lsp.DetectServer(dir)
	require.NoError(t, err)
	assert.Equal(t, "pyright", serverCmd)
	assert.Contains(t, args, "--stdio")
}

// TestDetectServer_PriorityGoOverTS는 go.mod와 package.json이 모두 있을 때 Go가 우선임을 테스트한다.
func TestDetectServer_PriorityGoOverTS(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeProjectFile(t, dir, "go.mod", "module test\n\ngo 1.23\n")
	writeProjectFile(t, dir, "package.json", `{"name":"test"}`)

	serverCmd, _, err := lsp.DetectServer(dir)
	require.NoError(t, err)
	assert.Equal(t, "gopls", serverCmd)
}
