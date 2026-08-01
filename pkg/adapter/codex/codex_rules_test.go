package codex_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentfs "github.com/insajin/autopus-adk/content"
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
		"shell-portability.md",
		"spec-quality.md",
		"subagent-delegation.md",
		"techstack-freshness.md",
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

	// Codex sources rules from content/rules via contentfs, so relocating a
	// claude-code rule body under .claude/hooks/autopus/conditional/ cannot
	// shrink this set (SPEC-CONDRULE-001 S7).
	sourceEntries, err := fs.ReadDir(contentfs.FS, "rules")
	require.NoError(t, err)
	assert.Len(t, sourceEntries, len(expectedRules),
		"codex rule count must track the content/rules source set")
}

// triggerFieldKeys are the omp-contract trigger fields that REQ-CONDRULE-SCHEMA-04
// requires to round-trip through emission uninterpreted.
var triggerFieldKeys = []string{"condition", "scope", "interruptMode", "astCondition"}

// frontmatterFields maps each top-level frontmatter key to its verbatim value
// text, without trimming, so byte-level drift is observable.
func frontmatterFields(t *testing.T, raw string) map[string]string {
	t.Helper()
	require.True(t, strings.HasPrefix(raw, "---\n"), "document must open with frontmatter")
	rest := raw[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	require.GreaterOrEqual(t, end, 0, "frontmatter must be closed")

	fields := map[string]string{}
	for _, line := range strings.Split(rest[:end], "\n") {
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields[key] = strings.TrimPrefix(value, " ")
	}
	return fields
}

// TestGenerateRuleFiles_PreservesTriggerFrontmatter locks SPEC-CONDRULE-001 S7 on
// the codex path: ensureCodexRulePlatform appends platform to an existing block
// and preserves every other key, including the trigger fields.
func TestGenerateRuleFiles_PreservesTriggerFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	sourceRaw, err := fs.ReadFile(contentfs.FS, "rules/lore-commit.md")
	require.NoError(t, err)
	source := frontmatterFields(t, string(sourceRaw))

	emittedRaw, err := os.ReadFile(filepath.Join(dir, ".codex", "rules", "autopus", "lore-commit.md"))
	require.NoError(t, err)
	emitted := frontmatterFields(t, string(emittedRaw))

	for _, key := range triggerFieldKeys {
		require.Contains(t, source, key,
			"content/rules/lore-commit.md must declare %s", key)
		assert.Equal(t, source[key], emitted[key],
			"codex must preserve the %s value verbatim", key)
	}

	assert.Equal(t, "codex", emitted["platform"],
		"codex frontmatter must carry platform: codex")
	for key, want := range source {
		assert.Equal(t, want, emitted[key],
			"codex must preserve frontmatter key %s", key)
	}
	assert.Len(t, emitted, len(source)+1,
		"codex must add only the platform key to the source frontmatter")
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
	assert.Contains(t, string(data), "SPEC Markdown files under `.autopus/specs/**`",
		"file-size-limit should explicitly exempt generated SPEC Markdown")
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

	techstackPath := filepath.Join(dir, ".codex", "rules", "autopus", "techstack-freshness.md")
	techstackData, err := os.ReadFile(techstackPath)
	require.NoError(t, err)
	assert.Contains(t, string(techstackData), "Technology Stack Decision")
	assert.Contains(t, string(techstackData), "greenfield")

	projectIdentityPath := filepath.Join(dir, ".codex", "rules", "autopus", "project-identity.md")
	projectIdentityData, err := os.ReadFile(projectIdentityPath)
	require.NoError(t, err)
	assert.Contains(t, string(projectIdentityData), "Do NOT confuse the user's project")

	shellPortabilityPath := filepath.Join(dir, ".codex", "rules", "autopus", "shell-portability.md")
	shellPortabilityData, err := os.ReadFile(shellPortabilityPath)
	require.NoError(t, err)
	assert.Contains(t, string(shellPortabilityData), "Do NOT prefix commands with GNU `timeout`")
	assert.Contains(t, string(shellPortabilityData), "exit code `127`")
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
	assert.Contains(t, content, "SPEC Markdown files under .autopus/specs/**",
		"AGENTS.md should prevent agents from applying the source line limit to SPEC docs")
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
