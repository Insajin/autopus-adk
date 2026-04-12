package codex_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestGenerateRuleFiles_ProducesSeven(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	expectedRules := []string{
		"context7-docs.md",
		"doc-storage.md",
		"file-size-limit.md",
		"language-policy.md",
		"lore-commit.md",
		"objective-reasoning.md",
		"subagent-delegation.md",
		"worktree-safety.md",
	}

	rulesDir := filepath.Join(dir, ".codex", "rules", "autopus")
	for _, rule := range expectedRules {
		rulePath := filepath.Join(rulesDir, rule)
		_, statErr := os.Stat(rulePath)
		assert.NoError(t, statErr, "rule file should exist: %s", rule)
	}

	// Verify exactly 7 rule files
	entries, err := os.ReadDir(rulesDir)
	require.NoError(t, err)
	assert.Len(t, entries, len(expectedRules), "should have exactly 7 rule files")
}

func TestGenerateRuleFiles_Content(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Verify file-size-limit has key content
	fsPath := filepath.Join(dir, ".codex", "rules", "autopus", "file-size-limit.md")
	data, err := os.ReadFile(fsPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "300 lines",
		"file-size-limit should reference 300 lines")
	assert.Contains(t, string(data), "platform: codex",
		"should have codex platform in frontmatter")

	// Verify lore-commit has key content
	lorePath := filepath.Join(dir, ".codex", "rules", "autopus", "lore-commit.md")
	loreData, err := os.ReadFile(lorePath)
	require.NoError(t, err)
	assert.Contains(t, string(loreData), "Lore Commit",
		"should contain rule title")
	assert.NotContains(t, string(loreData), "@import content/rules/",
		"codex rules should inline imported rule bodies")
	assert.NotContains(t, string(loreData), "# Lore Commit\n\n# Lore Commit",
		"codex rules should not duplicate the title after inlining")

	contextPath := filepath.Join(dir, ".codex", "rules", "autopus", "context7-docs.md")
	contextData, err := os.ReadFile(contextPath)
	require.NoError(t, err)
	assert.Contains(t, string(contextData), "Before any technology/library/framework-related work",
		"context7-docs should include imported rule content")
	assert.NotContains(t, string(contextData), "@import content/rules/",
		"context7-docs should not leave unresolved imports")
}

func TestAgentsMD_NoInlineRules(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	agentsPath := filepath.Join(dir, "AGENTS.md")
	data, err := os.ReadFile(agentsPath)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "See .codex/rules/autopus/ for detailed guidelines",
		"AGENTS.md should reference rules directory")
	assert.NotContains(t, content, "IMPORTANT: No single file may exceed 300 lines",
		"AGENTS.md should not contain inline file-size-limit rule")
}

func TestRuleFilePath_Flat(t *testing.T) {
	t.Parallel()
	// Flat fallback naming convention test.
	// When subdir support is disabled, paths should use flat naming.
	// Since detectCodexSubdirSupport() defaults to true, we verify
	// the subdirectory path is used.
	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Verify subdirectory structure exists (not flat)
	rulesDir := filepath.Join(dir, ".codex", "rules", "autopus")
	info, err := os.Stat(rulesDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), ".codex/rules/autopus/ should be a directory")
}
