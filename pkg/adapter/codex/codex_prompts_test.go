package codex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderPromptTemplates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.renderPromptTemplates(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, files, "should produce prompt file mappings")

	// All 7 prompts should be generated.
	assert.Len(t, files, 7)

	// Verify files written to disk.
	for _, f := range files {
		fullPath := filepath.Join(dir, f.TargetPath)
		assert.FileExists(t, fullPath)
	}
}

func TestRenderPromptTemplates_ContainsProjectName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("my-app")

	files, err := a.renderPromptTemplates(cfg)
	require.NoError(t, err)

	found := false
	for _, f := range files {
		if string(f.Content) != "" {
			if assert.Contains(t, string(f.Content), "my-app") {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "at least one prompt should contain project name")
}

func TestPreparePromptFiles_NoDiskWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.preparePromptFiles(cfg)
	require.NoError(t, err)
	assert.Len(t, files, 7)

	// preparePromptFiles should NOT write to disk.
	promptsDir := filepath.Join(dir, ".codex", "prompts")
	_, err = os.Stat(promptsDir)
	assert.True(t, os.IsNotExist(err), "preparePromptFiles should not create files on disk")
}

func TestRenderPromptTemplates_TargetPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.renderPromptTemplates(cfg)
	require.NoError(t, err)

	expectedPrefixes := filepath.Join(".codex", "prompts")
	for _, f := range files {
		assert.Contains(t, f.TargetPath, expectedPrefixes,
			"prompt target path should be under .codex/prompts/")
	}
}

func TestRenderPromptTemplates_YAMLFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.renderPromptTemplates(cfg)
	require.NoError(t, err)

	for _, f := range files {
		content := string(f.Content)
		assert.Contains(t, content, "---", "prompt %s should have YAML frontmatter", f.TargetPath)
		assert.Contains(t, content, "description:", "prompt %s should have description field", f.TargetPath)
	}
}
