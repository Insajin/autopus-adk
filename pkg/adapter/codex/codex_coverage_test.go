package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Extended Skills ---

func TestRenderExtendedSkills(t *testing.T) {
	t.Parallel()
	a := NewWithRoot(t.TempDir())
	files, err := a.renderExtendedSkills()
	require.NoError(t, err)
	assert.NotEmpty(t, files)
	for _, f := range files {
		assert.Contains(t, f.TargetPath, ".codex/skills/")
		assert.Equal(t, adapter.OverwriteAlways, f.OverwritePolicy)
	}
}

func TestNormalizeCodexExtendedSkill_RewritesSpecialSkills(t *testing.T) {
	t.Parallel()

	teams := normalizeCodexExtendedSkill("agent-teams", "placeholder")
	assert.Contains(t, teams, "spawn_agent")
	assert.NotContains(t, teams, "TeamCreate")
	assert.NotContains(t, teams, "SendMessage")

	pipeline := normalizeCodexExtendedSkill("agent-pipeline", "placeholder")
	assert.Contains(t, pipeline, "@auto go")
	assert.Contains(t, pipeline, "spawn_agent")
	assert.NotContains(t, pipeline, "bypassPermissions")
	assert.NotContains(t, pipeline, "auto permission detect")

	worktree := normalizeCodexExtendedSkill("worktree-isolation", "placeholder")
	assert.Contains(t, worktree, "forked workspace")
	assert.NotContains(t, worktree, "auto pipeline worktree")

	prd := normalizeCodexExtendedSkill("prd", "사용자 입력이 불충분할 경우 AskUserQuestion으로 확인:")
	assert.NotContains(t, prd, "AskUserQuestion")
	assert.Contains(t, prd, "plain-text")
}

func TestLogTransformReport_Nil(t *testing.T) {
	t.Parallel()
	logTransformReport("codex", nil)
}

func TestLogTransformReport_WithData(t *testing.T) {
	t.Parallel()
	report := &pkgcontent.TransformReport{
		Compatible:   []string{"skill-a", "skill-b"},
		Incompatible: []string{"skill-c"},
	}
	logTransformReport("codex", report)
}

// --- Hooks ---

func TestInstallGitHooks(t *testing.T) {
	t.Parallel()
	require.NoError(t, NewWithRoot(t.TempDir()).installGitHooks(config.DefaultFullConfig("test")))
}

func TestRenderHooksTemplate(t *testing.T) {
	t.Parallel()
	rendered, err := NewWithRoot(t.TempDir()).renderHooksTemplate(config.DefaultFullConfig("test"))
	require.NoError(t, err)
	assert.Contains(t, rendered, "SessionStart")
}

func TestGenerateHooks_WritesToDisk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files, err := NewWithRoot(dir).generateHooks(config.DefaultFullConfig("test"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	data, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
	require.NoError(t, err)
	assert.JSONEq(t, string(files[0].Content), string(data))
}

func TestPrepareHooksFile_MergesExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	hooksDir := filepath.Join(dir, ".codex")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "hooks.json"),
		[]byte(`{"hooks":{"CustomEvent":[{"command":"user.sh"}]}}`),
		0644,
	))

	files, err := a.prepareHooksFile(cfg)
	require.NoError(t, err)
	require.Len(t, files, 1)

	content := string(files[0].Content)
	assert.Contains(t, content, "user.sh", "user hook preserved")
	assert.Contains(t, content, "SessionStart", "autopus hooks added")
}

func TestMergeHooks_InvalidRenderedJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := mergeHooks(filepath.Join(dir, "x.json"), "{bad")
	assert.Error(t, err)
}

func TestMergeHookCategories_EmptyDocs(t *testing.T) {
	t.Parallel()
	empty := hooksDoc{Hooks: map[string][]hookEntry{}}
	result := mergeHookCategories(empty, empty)
	assert.NotNil(t, result.Hooks)
	assert.Empty(t, result.Hooks)
}

// --- Settings ---

func TestGenerateConfig_WritesToDisk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateConfig(cfg)
	require.NoError(t, err)
	require.Len(t, files, 1)

	data, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	require.NoError(t, err)
	assert.Equal(t, string(files[0].Content), string(data))
}

// --- Lifecycle ---

func TestValidate_MarkerPresentButNoSkills(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	content := "# Test\n" + markerBegin + "\ncontent\n" + markerEnd + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0644))

	errs, err := a.Validate(context.Background())
	require.NoError(t, err)

	found := false
	for _, e := range errs {
		if e.Level == "error" && e.File == ".codex/skills" {
			found = true
		}
	}
	assert.True(t, found, "should report missing .codex/skills")
}

func TestValidate_NoMarkerSection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "AGENTS.md"),
		[]byte("# No Marker\n"),
		0644,
	))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "skills"), 0755))

	errs, err := a.Validate(context.Background())
	require.NoError(t, err)

	found := false
	for _, e := range errs {
		if e.Level == "warning" && e.File == "AGENTS.md" {
			found = true
		}
	}
	assert.True(t, found, "should warn about missing marker")
}

// --- prepareFiles ---

func TestPrepareFiles_ReturnsAllCategories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.prepareFiles(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, files)

	hasAgentsMD := false
	hasSkills := false
	hasRules := false
	hasHooks := false
	hasConfig := false

	for _, f := range files {
		switch {
		case f.TargetPath == "AGENTS.md":
			hasAgentsMD = true
		case strings.Contains(f.TargetPath, "skills"):
			hasSkills = true
		case strings.Contains(f.TargetPath, "rules"):
			hasRules = true
		case strings.Contains(f.TargetPath, "hooks"):
			hasHooks = true
		case f.TargetPath == "config.toml":
			hasConfig = true
		}
	}

	assert.True(t, hasAgentsMD, "should have AGENTS.md")
	assert.True(t, hasSkills, "should have skill files")
	assert.True(t, hasRules, "should have rule files")
	assert.True(t, hasHooks, "should have hooks file")
	assert.True(t, hasConfig, "should have config file")
}

// --- Render functions ---

func TestRenderSkillTemplates_WritesAndReturns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "skills"), 0755))

	files, err := a.renderSkillTemplates(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, files)
}

func TestRenderPromptTemplates_WritesAllPrompts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.renderPromptTemplates(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, files)
}

func TestGenerateRuleFiles_WritesToDisk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateRuleFiles(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, files)
}

func TestGenerateAgents_WritesAndReturns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateAgents(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, files)
}

// --- Prepare helpers ---

func TestPrepareHooksFile_MergeResult(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	files, err := a.prepareHooksFile(cfg)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, adapter.OverwriteMerge, files[0].OverwritePolicy)
}

func TestClean_OnlyAgentsMD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	content := "header\n" + markerBegin + "\ncontent\n" + markerEnd + "\nfooter"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0644))
	require.NoError(t, a.Clean(context.Background()))
	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	assert.NotContains(t, string(data), markerBegin)
	assert.Contains(t, string(data), "header")
}

func TestClean_UnreadableAgentsMD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	agentsPath := filepath.Join(dir, "AGENTS.md")
	require.NoError(t, os.WriteFile(agentsPath, []byte("content"), 0000))
	t.Cleanup(func() { os.Chmod(agentsPath, 0644) })
	err := a.Clean(context.Background())
	if err != nil {
		assert.Contains(t, err.Error(), "AGENTS.md")
	}
}
