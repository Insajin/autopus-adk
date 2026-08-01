package claude_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	claudeRulesRelDir     = ".claude/rules/autopus"
	conditionalBodyRelDir = ".claude/hooks/autopus/conditional"
	conditionalManifest   = ".claude/hooks/autopus/conditional-rules.json"
)

// baselineRuleFiles is the exact S3 baseline set that stays in context at
// session start after relocation.
var baselineRuleFiles = []string{
	"branding.md",
	"context7-docs.md",
	"deferred-tools.md",
	"doc-storage.md",
	"file-size-limit.md",
	"language-policy.md",
	"objective-reasoning.md",
	"project-identity.md",
	"spec-quality.md",
	"subagent-delegation.md",
	"techstack-freshness.md",
}

var hookFiredRuleFiles = []string{"lore-commit.md", "shell-portability.md", "worktree-safety.md"}

// generateClaudeSurface runs the claude adapter into a fresh root and returns it.
func generateClaudeSurface(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_, err := claude.NewWithRoot(dir).Generate(context.Background(), config.DefaultFullConfig("condrule"))
	require.NoError(t, err)
	return dir
}

func readDirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "directory must exist: %s", dir)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}

// splitFrontmatterBlock returns the frontmatter body and the remaining content.
func splitFrontmatterBlock(raw string) (frontmatter, body string) {
	if !strings.HasPrefix(raw, "---\n") {
		return "", raw
	}
	rest := raw[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", raw
	}
	return rest[:end], rest[end+len("\n---\n"):]
}

// stripFrontmatter removes a leading YAML frontmatter block.
func stripFrontmatter(raw string) string {
	_, body := splitFrontmatterBlock(raw)
	return body
}

// TestClaudeGenerate_HookFiredRuleBodiesLeaveBaselineContext is the S3 oracle
// for REQ-CONDRULE-COMPILE-02 and INV-003.
func TestClaudeGenerate_HookFiredRuleBodiesLeaveBaselineContext(t *testing.T) {
	t.Parallel()
	dir := generateClaudeSurface(t)

	assert.Equal(t, baselineRuleFiles, readDirNames(t, filepath.Join(dir, claudeRulesRelDir)),
		"baseline rule set after relocation")

	for _, name := range hookFiredRuleFiles {
		_, err := os.Stat(filepath.Join(dir, claudeRulesRelDir, name))
		assert.True(t, os.IsNotExist(err), "%s must not load at session start", name)
	}

	assert.Equal(t, hookFiredRuleFiles, readDirNames(t, filepath.Join(dir, conditionalBodyRelDir)))

	for _, name := range hookFiredRuleFiles {
		source, err := fs.ReadFile(contentfs.FS, "rules/"+name)
		require.NoError(t, err)
		relocated, err := os.ReadFile(filepath.Join(dir, conditionalBodyRelDir, name))
		require.NoError(t, err)

		assert.Equal(t, strings.TrimSpace(stripFrontmatter(string(source))),
			strings.TrimSpace(string(relocated)),
			"%s body must match the source body after frontmatter removal", name)
		assert.False(t, strings.HasPrefix(strings.TrimSpace(string(relocated)), "---"),
			"%s must ship without frontmatter", name)
	}
}

// TestClaudeGenerate_PathsScopedRuleUsesNativeFrontmatter is the S13 oracle for
// REQ-CONDRULE-COMPILE-01.
func TestClaudeGenerate_PathsScopedRuleUsesNativeFrontmatter(t *testing.T) {
	t.Parallel()
	dir := generateClaudeSurface(t)

	path := filepath.Join(dir, claudeRulesRelDir, "file-size-limit.md")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(raw)

	frontmatter, _ := splitFrontmatterBlock(content)
	require.NotEmpty(t, frontmatter, "file-size-limit must carry frontmatter")
	assert.Contains(t, frontmatter, "paths:", "globs compile to a native paths: list")
	assert.Contains(t, frontmatter, "**/*.go")

	assert.Contains(t, content, "300 lines",
		"the dynamic file-size threshold rendering must survive")

	_, err = os.Stat(filepath.Join(dir, conditionalBodyRelDir, "file-size-limit.md"))
	assert.True(t, os.IsNotExist(err), "a paths-scoped rule is not relocated")

	manifest, err := os.ReadFile(filepath.Join(dir, conditionalManifest))
	require.NoError(t, err)
	assert.NotContains(t, string(manifest), "file-size-limit",
		"a paths-scoped rule gets no manifest entry")
}
