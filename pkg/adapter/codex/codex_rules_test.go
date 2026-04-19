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

func TestGenerateRuleFiles_ProducesManagedRuleSet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	expectedRules := []string{
		"branding.md",
		"context7-docs.md",
		"deferred-tools.md",
		"doc-storage.md",
		"file-size-limit.md",
		"language-policy.md",
		"lore-commit.md",
		"objective-reasoning.md",
		"project-identity.md",
		"spec-quality.md",
		"subagent-delegation.md",
		"worktree-safety.md",
	}

	rulesDir := filepath.Join(dir, ".codex", "rules", "autopus")
	for _, rule := range expectedRules {
		rulePath := filepath.Join(rulesDir, rule)
		_, statErr := os.Stat(rulePath)
		assert.NoError(t, statErr, "rule file should exist: %s", rule)
	}

	// Verify the full managed rule set is present.
	entries, err := os.ReadDir(rulesDir)
	require.NoError(t, err)
	assert.Len(t, entries, len(expectedRules), "should have the full managed rule set")
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
		"managed rules should render concrete rule bodies, not stub imports")

	brandingPath := filepath.Join(dir, ".codex", "rules", "autopus", "branding.md")
	brandingData, err := os.ReadFile(brandingPath)
	require.NoError(t, err)
	assert.Contains(t, string(brandingData), "Autopus Branding")

	context7Path := filepath.Join(dir, ".codex", "rules", "autopus", "context7-docs.md")
	context7Data, err := os.ReadFile(context7Path)
	require.NoError(t, err)
	assert.Contains(t, string(context7Data), "web search")
	assert.Contains(t, string(context7Data), "official docs")

	projectIdentityPath := filepath.Join(dir, ".codex", "rules", "autopus", "project-identity.md")
	projectIdentityData, err := os.ReadFile(projectIdentityPath)
	require.NoError(t, err)
	assert.Contains(t, string(projectIdentityData), "Do NOT confuse the user's project")
}

func TestAgentsMD_IncludesCoreCodexGuidance(t *testing.T) {
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

	assert.Contains(t, content, "## Core Guidelines",
		"AGENTS.md should inline the key Codex operating rules")
	assert.Contains(t, content, "### Subagent Delegation",
		"AGENTS.md should preserve delegation policy")
	assert.Contains(t, content, "### Review Convergence",
		"AGENTS.md should preserve review convergence guidance")
	assert.Contains(t, content, "See .codex/rules/autopus/ for Codex rule definitions.",
		"AGENTS.md should reference rules directory")
	assert.Contains(t, content, ".codex/skills/agent-pipeline.md",
		"AGENTS.md should point to the pipeline contract")
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
