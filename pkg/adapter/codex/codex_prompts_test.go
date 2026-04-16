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

	// Router + 16 workflow prompts should be generated.
	assert.Len(t, files, 17)

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
	assert.Len(t, files, 17)

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

func TestRenderPromptTemplates_WorkflowContractsPresent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.renderPromptTemplates(cfg)
	require.NoError(t, err)

	byName := map[string]string{}
	for _, f := range files {
		byName[filepath.Base(f.TargetPath)] = string(f.Content)
	}

	assert.Contains(t, byName["auto-idea.md"], "orchestra CLI를 반드시 먼저 호출")
	assert.Contains(t, byName["auto-status.md"], "auto status")
	assert.Contains(t, byName["auto-setup.md"], "explorer")
	assert.Contains(t, byName["auto-setup.md"], "ARCHITECTURE.md")
	assert.Contains(t, byName["auto-plan.md"], "auto spec review {SPEC-ID}")
	assert.Contains(t, byName["auto-go.md"], "draft")
	assert.Contains(t, byName["auto-go.md"], "max_revisions")
	assert.Contains(t, byName["auto-go.md"], "approved")
	assert.Contains(t, byName["auto-go.md"], "재귀 호출")
	assert.Contains(t, byName["auto-go.md"], "## SPEC Path Resolution")
	assert.Contains(t, byName["auto-go.md"], "WORKING_DIR")
	assert.Contains(t, byName["auto-go.md"], "## Completion Handoff Gates")
	assert.Contains(t, byName["auto-go.md"], "`next_required_step`")
	assert.Contains(t, byName["auto-go.md"], "`next_command`")
	assert.Contains(t, byName["auto-go.md"], "`auto_progression_state`")
	assert.Contains(t, byName["auto-go.md"], "workflow lifecycle bar를 먼저 보여준 뒤")
	assert.Contains(t, byName["auto-sync.md"], "@AX lifecycle")
	assert.Contains(t, byName["auto-sync.md"], "## Completion Gates")
	assert.Contains(t, byName["auto-sync.md"], "@AX: no-op")
	assert.Contains(t, byName["auto-sync.md"], "commit hash")
	assert.Contains(t, byName["auto-sync.md"], "sync completed 선언을 금지")
	assert.Contains(t, byName["auto-map.md"], "spawn_agent")
	assert.Contains(t, byName["auto-why.md"], "auto lore context <path>")
	assert.Contains(t, byName["auto-verify.md"], "auto verify")
	assert.Contains(t, byName["auto-secure.md"], "OWASP Top 10")
	assert.Contains(t, byName["auto-test.md"], "auto test run")
	assert.Contains(t, byName["auto-dev.md"], "`auto-plan`")
	assert.Contains(t, byName["auto-doctor.md"], "auto doctor")
	assert.Contains(t, byName["auto.md"], "## Autopus Branding")
	assert.Contains(t, byName["auto.md"], "## Router Execution Contract")
	assert.Contains(t, byName["auto.md"], "## Context Load")
	assert.Contains(t, byName["auto.md"], "## SPEC Path Resolution")
	assert.Contains(t, byName["auto.md"], "ARCHITECTURE.md")
}

func TestRenderPromptTemplates_AllPromptsIncludeBranding(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.renderPromptTemplates(cfg)
	require.NoError(t, err)

	for _, f := range files {
		content := string(f.Content)
		assert.Contains(t, content, "## Autopus Branding", f.TargetPath)
		assert.Contains(t, content, "🐙 Autopus ─────────────────────────", f.TargetPath)
	}
}
