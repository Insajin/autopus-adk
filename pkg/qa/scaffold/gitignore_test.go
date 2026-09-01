package scaffold

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
)

func TestInitEmitsQARuntimeGitignoreCoveringRawCaptureBytes(t *testing.T) {
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, ".git"))
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26\n")

	result, err := Init(Options{ProjectDir: dir, ProjectDirExplicit: true})
	require.NoError(t, err)

	assertCreatedID(t, result, qaRuntimeGitignoreID)
	path := filepath.Join(dir, ".autopus", "qa", ".gitignore")
	require.FileExists(t, path)
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	// The pattern has to cover the real pre-redaction layout the runner allocates,
	// not a hand-written guess at it.
	rawDir := capture.LocalCaptureDir(filepath.Join("runs", "qa-0001", "go-fast"))
	assert.True(t, gitignoreCovers(string(body), rawDir), "%q must ignore %q", string(body), rawDir)
	assert.True(t, gitignoreCovers(string(body), filepath.Join("cache", "sandbox-home", "Library", "state")))
	assert.False(t, gitignoreCovers(string(body), filepath.Join("runs", "qa-0001", "go-fast", "manifest.json")),
		"redacted published evidence must stay trackable")
}

func TestInitPreservesEditedQARuntimeGitignore(t *testing.T) {
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, ".git"))
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.26\n")
	_, err := Init(Options{ProjectDir: dir, ProjectDirExplicit: true})
	require.NoError(t, err)
	path := filepath.Join(dir, ".autopus", "qa", ".gitignore")
	edited := "_raw/\n# operator addition\nlocal-notes/\n"
	require.NoError(t, os.WriteFile(path, []byte(edited), 0o644))

	result, err := Init(Options{ProjectDir: dir, ProjectDirExplicit: true})
	require.NoError(t, err)

	assertSkippedID(t, result, qaRuntimeGitignoreID)
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, edited, string(body))
}

func TestInitWithoutQASignalsEmitsNoGitignore(t *testing.T) {
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, ".git"))

	result, err := Init(Options{ProjectDir: dir, ProjectDirExplicit: true})
	require.NoError(t, err)

	assert.Equal(t, "noop", result.Status)
	assert.Empty(t, result.Created)
	assert.NoFileExists(t, filepath.Join(dir, ".autopus", "qa", ".gitignore"))
}

// gitignoreCovers evaluates the generated patterns the way git does for the
// directory-suffixed entries this file emits: a `name/` pattern ignores any path
// with that directory component.
func gitignoreCovers(body, relPath string) bool {
	components := strings.Split(filepath.ToSlash(relPath), "/")
	for _, line := range strings.Split(body, "\n") {
		pattern, ok := directoryPattern(line)
		if !ok {
			continue
		}
		if slices.Contains(components, pattern) {
			return true
		}
	}
	return false
}

func directoryPattern(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.HasSuffix(trimmed, "/") {
		return "", false
	}
	return strings.TrimSuffix(trimmed, "/"), true
}
