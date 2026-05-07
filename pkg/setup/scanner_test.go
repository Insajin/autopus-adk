package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScan_GoProject(t *testing.T) {
	t.Parallel()
	dir := setupGoProject(t)

	info, err := Scan(dir)
	require.NoError(t, err)

	assert.Equal(t, filepath.Base(dir), info.Name)

	// Languages
	require.Len(t, info.Languages, 1)
	assert.Equal(t, "Go", info.Languages[0].Name)
	assert.Equal(t, "1.23", info.Languages[0].Version)

	// Build files
	assert.NotEmpty(t, info.BuildFiles)
	var hasGoMod bool
	for _, bf := range info.BuildFiles {
		if bf.Type == "go.mod" {
			hasGoMod = true
			assert.Contains(t, bf.Commands, "build")
			assert.Contains(t, bf.Commands, "test")
		}
	}
	assert.True(t, hasGoMod, "should detect go.mod as build file")

	// Entry points
	assert.NotEmpty(t, info.EntryPoints)

	// Test config
	assert.Equal(t, "go test", info.TestConfig.Framework)
	assert.Contains(t, info.TestConfig.Command, "go test")

	// Structure (max 3 levels)
	assert.NotEmpty(t, info.Structure)
}

func TestScan_JSProject(t *testing.T) {
	t.Parallel()
	dir := setupJSProject(t)

	info, err := Scan(dir)
	require.NoError(t, err)

	require.Len(t, info.Languages, 1)
	assert.Equal(t, "JavaScript", info.Languages[0].Name)

	// Build files from package.json
	var hasPkgJSON bool
	for _, bf := range info.BuildFiles {
		if bf.Type == "package.json" {
			hasPkgJSON = true
			assert.Contains(t, bf.Commands, "test")
			assert.Contains(t, bf.Commands, "build")
		}
	}
	assert.True(t, hasPkgJSON)
}

func TestScan_MultiLanguageProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Go + JS
	writeFile(t, dir, "go.mod", "module example\n\ngo 1.23\n")
	writeFile(t, dir, "cmd/app/main.go", "package main\nfunc main() {}\n")
	writeFile(t, dir, "package.json", `{"scripts":{"test":"jest"}}`)

	info, err := Scan(dir)
	require.NoError(t, err)
	assert.Len(t, info.Languages, 2)
}

func TestScan_EmptyProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	info, err := Scan(dir)
	require.NoError(t, err)
	assert.Empty(t, info.Languages)
	assert.Empty(t, info.BuildFiles)
}

func TestScan_WithMakefile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, "Makefile", "build:\n\tgo build ./...\n\ntest:\n\tgo test ./...\n\nlint:\n\tgolangci-lint run\n")
	writeFile(t, dir, "go.mod", "module example\n\ngo 1.23\n")

	info, err := Scan(dir)
	require.NoError(t, err)

	var hasMakefile bool
	for _, bf := range info.BuildFiles {
		if bf.Type == "makefile" {
			hasMakefile = true
			assert.Contains(t, bf.Commands, "build")
			assert.Contains(t, bf.Commands, "test")
			assert.Contains(t, bf.Commands, "lint")
		}
	}
	assert.True(t, hasMakefile)
}

func TestScan_DirectoryTreeMaxDepth(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create 5 levels deep
	writeFile(t, dir, "a/b/c/d/e/deep.txt", "deep")

	info, err := Scan(dir)
	require.NoError(t, err)

	// Verify directory tree depth is bounded to maxDepth levels
	var depth int
	measureDepth(info.Structure, 1, &depth)
	assert.LessOrEqual(t, depth, maxDepth)

	// Verify deep directories are excluded
	assert.False(t, hasDirNamed(info.Structure, "d"), "should not contain dir 'd'")
	assert.False(t, hasDirNamed(info.Structure, "e"), "should not contain dir 'e'")
}

func TestDetectFrameworks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	pkgJSON := map[string]any{
		"dependencies": map[string]string{
			"react":   "^18.0.0",
			"express": "^4.18.0",
		},
	}
	data, _ := json.Marshal(pkgJSON)
	writeFile(t, dir, "package.json", string(data))

	frameworks := detectFrameworks(dir)
	assert.Len(t, frameworks, 2)

	names := make(map[string]bool)
	for _, f := range frameworks {
		names[f.Name] = true
	}
	assert.True(t, names["React"])
	assert.True(t, names["Express"])
}

func TestDetectFrameworksUsesSharedDetectorSignals(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "next.config.ts", "// next config")

	frameworks := detectFrameworks(dir)

	require.Len(t, frameworks, 1)
	assert.Equal(t, "Next.js", frameworks[0].Name)
}

func TestDetectFrameworksIncludesGoAndPythonFrameworks(t *testing.T) {
	t.Parallel()

	goDir := t.TempDir()
	writeFile(t, goDir, "go.mod", "module example.com/api\n\ngo 1.23\n\nrequire github.com/gin-gonic/gin v1.10.0\n")
	goFrameworks := detectFrameworks(goDir)
	require.Len(t, goFrameworks, 1)
	assert.Equal(t, "Gin", goFrameworks[0].Name)
	assert.Equal(t, "v1.10.0", goFrameworks[0].Version)

	pyDir := t.TempDir()
	writeFile(t, pyDir, "pyproject.toml", "[project]\ndependencies = [\n  \"fastapi>=0.110.0\",\n]\n")
	pyFrameworks := detectFrameworks(pyDir)
	require.Len(t, pyFrameworks, 1)
	assert.Equal(t, "FastAPI", pyFrameworks[0].Name)
	assert.Equal(t, "0.110.0", pyFrameworks[0].Version)
}

// --- helpers ---

func setupGoProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, dir, "go.mod", "module example\n\ngo 1.23\n")
	writeFile(t, dir, "cmd/app/main.go", "package main\n\nfunc main() {}\n")
	writeFile(t, dir, "pkg/util/util.go", "package util\n")
	writeFile(t, dir, "pkg/util/util_test.go", "package util\n")
	writeFile(t, dir, "internal/core/core.go", "package core\n")

	return dir
}

func setupJSProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	pkgJSON := map[string]any{
		"name": "test-project",
		"scripts": map[string]string{
			"test":  "jest",
			"build": "tsc",
			"lint":  "eslint .",
		},
	}
	data, _ := json.Marshal(pkgJSON)
	writeFile(t, dir, "package.json", string(data))
	writeFile(t, dir, "src/index.js", "console.log('hello')\n")

	return dir
}

func writeFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(dir, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
}

func measureDepth(entries []DirEntry, current int, max *int) {
	if len(entries) == 0 {
		return
	}
	if current > *max {
		*max = current
	}
	for _, e := range entries {
		measureDepth(e.Children, current+1, max)
	}
}

func hasDirNamed(entries []DirEntry, name string) bool {
	for _, e := range entries {
		if e.Name == name {
			return true
		}
		if hasDirNamed(e.Children, name) {
			return true
		}
	}
	return false
}
