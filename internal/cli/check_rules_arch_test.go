package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckArch_WarnRangeFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeGoFileWithComments(t, dir, "warn.go", 200)

	var buf bytes.Buffer
	result := checkArch(dir, &buf, false, false)
	assert.True(t, result, "warn-range file must not fail arch check")
	assert.Contains(t, buf.String(), "consider splitting")
}

func TestCheckArch_WarnRangeQuiet(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeGoFileWithComments(t, dir, "warn.go", 200)

	var buf bytes.Buffer
	result := checkArch(dir, &buf, true, false)
	assert.True(t, result, "warn-range file must pass in quiet mode")
	assert.Empty(t, buf.String(), "quiet mode must suppress warn-range output")
}

func TestCheckArchStaged_OnlyChecksStaged(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	initTestGitRepo(t, dir)
	writeGoFileWithComments(t, dir, "big.go", 300)
	writeTestFile(t, dir, "small.go", "package dummy\n")

	runGitCommand(t, dir, "add", "small.go")

	var buf bytes.Buffer
	result := checkArch(dir, &buf, true, true)
	assert.True(t, result, "staged mode should pass when only small.go is staged")
}

func TestCheckArchStaged_FailsOnOversizedStaged(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	initTestGitRepo(t, dir)
	writeGoFileWithComments(t, dir, "big.go", 300)

	runGitCommand(t, dir, "add", "big.go")

	var buf bytes.Buffer
	result := checkArch(dir, &buf, true, true)
	assert.False(t, result, "staged mode should fail when big.go is staged")
	assert.Contains(t, buf.String(), "big.go")
}

func TestCheckArchWalk_SkipsSubmodule(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	subDir := filepath.Join(dir, "mysubmodule")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(
		t,
		os.WriteFile(filepath.Join(subDir, ".git"), []byte("gitdir: ../.git/modules/mysubmodule"), 0o644),
	)
	writeGoFileWithComments(t, subDir, "big.go", 300)

	var buf bytes.Buffer
	result := checkArch(dir, &buf, true, false)
	assert.True(t, result, "walk should skip submodule directories")
}

func TestCountLines_ReturnsCorrectCount(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeTestFile(t, dir, "test.go", "line1\nline2\nline3\n")

	n, err := countLines(path)
	require.NoError(t, err)
	assert.Equal(t, 3, n)
}

func TestCountLines_NonExistentFile(t *testing.T) {
	t.Parallel()

	_, err := countLines("/nonexistent/path/file.go")
	assert.Error(t, err)
}
