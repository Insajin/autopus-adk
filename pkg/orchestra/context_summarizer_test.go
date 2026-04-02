package orchestra

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/test/myproject\n\ngo 1.22\n"), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "ARCHITECTURE.md"), []byte(`# Architecture

This project provides a CLI tool for orchestration.

## Components

- `+"`pkg/core`"+` — core logic
- `+"`pkg/api`"+` — REST API layer
- `+"`cmd/cli`"+` — command-line interface
`), 0644))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".autopus/project"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".autopus/project/product.md"),
		[]byte("# Product\n\nA multi-provider orchestration engine for AI coding agents.\n"), 0644))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg/core"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd/cli"), 0755))

	return dir
}

func TestNewContextSummarizer_DefaultTokens(t *testing.T) {
	t.Parallel()
	cs := NewContextSummarizer(ContextSummarizerConfig{ProjectDir: t.TempDir()})
	assert.Equal(t, 2000, cs.cfg.MaxTokens)
}

func TestNewContextSummarizer_CustomTokens(t *testing.T) {
	t.Parallel()
	cs := NewContextSummarizer(ContextSummarizerConfig{ProjectDir: t.TempDir(), MaxTokens: 500})
	assert.Equal(t, 500, cs.cfg.MaxTokens)
}

func TestContextSummarizer_Summarize(t *testing.T) {
	t.Parallel()
	dir := setupTestProject(t)
	cs := NewContextSummarizer(ContextSummarizerConfig{ProjectDir: dir})

	data := cs.Summarize()

	assert.Equal(t, "myproject", data.ProjectName)
	assert.Contains(t, data.ProjectSummary, "multi-provider orchestration")
	assert.Equal(t, "Go", data.TechStack)
	assert.NotEmpty(t, data.MustReadFiles)
	assert.Contains(t, data.MustReadFiles, "ARCHITECTURE.md")
	assert.Contains(t, data.MustReadFiles, "go.mod")
}

func TestContextSummarizer_Components(t *testing.T) {
	t.Parallel()
	dir := setupTestProject(t)
	cs := NewContextSummarizer(ContextSummarizerConfig{ProjectDir: dir})

	data := cs.Summarize()

	assert.NotEmpty(t, data.Components)
	found := false
	for _, c := range data.Components {
		if c == "pkg/core" || c == "pkg/api" || c == "cmd/cli" {
			found = true
			break
		}
	}
	assert.True(t, found, "should find at least one component from ARCHITECTURE.md")
}

func TestContextSummarizer_ScanRelevantPaths(t *testing.T) {
	t.Parallel()
	dir := setupTestProject(t)
	cs := NewContextSummarizer(ContextSummarizerConfig{ProjectDir: dir})

	paths := cs.ScanRelevantPaths("test topic")

	assert.NotEmpty(t, paths)
	pathNames := make([]string, len(paths))
	for i, p := range paths {
		pathNames[i] = p.Path
	}
	assert.Contains(t, pathNames, "pkg")
	assert.Contains(t, pathNames, "cmd")
}

func TestContextSummarizer_PopulatePromptData(t *testing.T) {
	t.Parallel()
	dir := setupTestProject(t)
	cs := NewContextSummarizer(ContextSummarizerConfig{ProjectDir: dir})

	data := cs.PopulatePromptData("improve error handling")

	assert.Equal(t, "improve error handling", data.Topic)
	assert.Equal(t, "myproject", data.ProjectName)
	assert.NotEmpty(t, data.RelevantPaths)
	assert.Equal(t, 20, data.MaxTurns)
}

func TestContextSummarizer_EmptyProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cs := NewContextSummarizer(ContextSummarizerConfig{ProjectDir: dir})

	data := cs.Summarize()

	assert.Equal(t, filepath.Base(dir), data.ProjectName)
	assert.Empty(t, data.MustReadFiles)
}

func TestContextSummarizer_TokenBudget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a large ARCHITECTURE.md (much larger than budget)
	big := "# Architecture\n\n" + string(make([]byte, 50000))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ARCHITECTURE.md"), []byte(big), 0644))

	cs := NewContextSummarizer(ContextSummarizerConfig{ProjectDir: dir, MaxTokens: 100})
	data := cs.Summarize()

	// Should still produce results without crashing
	assert.NotEmpty(t, data.MustReadFiles)
}

func TestEstimateTokens(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 0, EstimateTokens(""))
	assert.Equal(t, 3, EstimateTokens("hello world"))
	assert.Equal(t, 25, EstimateTokens(string(make([]byte, 100))))
}

func TestTruncateToTokens(t *testing.T) {
	t.Parallel()

	t.Run("short string unchanged", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "hello", TruncateToTokens("hello", 100))
	})

	t.Run("long string truncated", func(t *testing.T) {
		t.Parallel()
		long := string(make([]byte, 1000))
		result := TruncateToTokens(long, 10)
		assert.Less(t, len(result), 100)
		assert.Contains(t, result, "truncated")
	})
}

func TestExtractFirstParagraph(t *testing.T) {
	t.Parallel()

	t.Run("skips headings", func(t *testing.T) {
		t.Parallel()
		content := "# Title\n\nThis is the first paragraph.\n\nSecond paragraph."
		assert.Equal(t, "This is the first paragraph.", extractFirstParagraph(content))
	})

	t.Run("empty content", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", extractFirstParagraph(""))
	})

	t.Run("truncates long paragraphs", func(t *testing.T) {
		t.Parallel()
		long := "# Title\n\n" + string(make([]byte, 600))
		result := extractFirstParagraph(long)
		assert.LessOrEqual(t, len(result), 500)
	})
}
