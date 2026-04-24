package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Generate error paths ---

func TestGenerate_FailsOnSkillsDirBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	codexDir := filepath.Join(dir, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(codexDir, "skills"), []byte("blocker"), 0444))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.Generate(context.Background(), cfg)
	assert.Error(t, err)
}

func TestGenerate_FailsOnAgentsMDWriteBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "skills"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "AGENTS.md"), 0755))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.Generate(context.Background(), cfg)
	assert.Error(t, err)
}

func TestGenerate_FailsOnSkillTemplateWriteBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, ".codex", "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(skillsDir, "auto-plan.md"), 0755))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.Generate(context.Background(), cfg)
	assert.Error(t, err)
}

func TestGenerate_FailsOnRulesDirBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "skills"), 0755))
	rulesParent := filepath.Join(dir, ".codex", "rules", "autopus")
	require.NoError(t, os.MkdirAll(filepath.Dir(rulesParent), 0755))
	require.NoError(t, os.WriteFile(rulesParent, []byte("blocker"), 0444))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.Generate(context.Background(), cfg)
	assert.Error(t, err)
}

func TestGenerate_FailsOnPromptWriteBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "skills"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".codex", "prompts"), []byte("blocker"), 0444))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.Generate(context.Background(), cfg)
	assert.Error(t, err)
}

func TestGenerate_FailsOnAgentWriteBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "skills"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".codex", "agents"), []byte("blocker"), 0444))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.Generate(context.Background(), cfg)
	assert.Error(t, err)
}

func TestGenerate_FailsOnHooksWriteBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "skills"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "agents"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "rules", "autopus"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "hooks.json"), 0755))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.Generate(context.Background(), cfg)
	assert.Error(t, err)
}

func TestGenerate_FailsOnConfigWriteBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "skills"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "config.toml"), 0755))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.Generate(context.Background(), cfg)
	assert.Error(t, err)
}

// --- Sub-function error paths ---

func TestGenerateRuleFiles_FailsOnReadOnlyTarget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rulesParent := filepath.Join(dir, ".codex", "rules", "autopus")
	require.NoError(t, os.MkdirAll(filepath.Dir(rulesParent), 0755))
	require.NoError(t, os.WriteFile(rulesParent, []byte("blocker"), 0444))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.generateRuleFiles(cfg)
	assert.Error(t, err)
}

func TestGenerateRuleFiles_WriteFileBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".codex", "rules", "autopus")
	require.NoError(t, os.MkdirAll(rulesDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(rulesDir, "lore-commit.md"), 0755))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.generateRuleFiles(cfg)
	assert.Error(t, err)
}

func TestRenderPromptTemplates_FailsOnReadOnlyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	promptsParent := filepath.Join(dir, ".codex", "prompts")
	require.NoError(t, os.MkdirAll(filepath.Dir(promptsParent), 0755))
	require.NoError(t, os.WriteFile(promptsParent, []byte("blocker"), 0444))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.renderPromptTemplates(cfg)
	assert.Error(t, err)
}

func TestRenderPromptTemplates_WriteFileBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	promptsDir := filepath.Join(dir, ".codex", "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(promptsDir, "auto-plan.md"), 0755))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.renderPromptTemplates(cfg)
	assert.Error(t, err)
}

func TestGenerateAgents_FailsOnReadOnlyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	agentsParent := filepath.Join(dir, ".codex", "agents")
	require.NoError(t, os.MkdirAll(filepath.Dir(agentsParent), 0755))
	require.NoError(t, os.WriteFile(agentsParent, []byte("blocker"), 0444))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.generateAgents(cfg)
	assert.Error(t, err)
}

func TestGenerateAgents_WriteFileBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".codex", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(agentsDir, "executor.toml"), 0755))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.generateAgents(cfg)
	assert.Error(t, err)
}

func TestRenderSkillTemplates_WriteFileBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, ".codex", "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(skillsDir, "auto-plan.md"), 0755))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.renderSkillTemplates(cfg)
	assert.Error(t, err)
}

func TestGenerateConfig_WriteFileBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "config.toml"), 0755))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.generateConfig(cfg)
	assert.Error(t, err)
}

func TestGenerateHooks_WriteFileBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".codex")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(hooksDir, "hooks.json"), 0755))

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.generateHooks(cfg)
	assert.Error(t, err)
}

func TestGenerateHooks_MkdirBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	codexParent := filepath.Join(dir, ".codex")
	require.NoError(t, os.WriteFile(codexParent, []byte("blocker"), 0444))
	t.Cleanup(func() { _ = os.Remove(codexParent) })

	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	_, err := a.generateHooks(cfg)
	assert.Error(t, err)
}

func TestInstallGitHooks_WriteFileBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	gitHooksDir := filepath.Join(dir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(gitHooksDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(gitHooksDir, "pre-commit"), 0755))

	err := a.installGitHooks(cfg)
	if err != nil {
		assert.Contains(t, err.Error(), "hook")
	}
}

func TestInstallGitHooks_MkdirBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	// Block .git/hooks as a file so MkdirAll for parent fails
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "hooks"), []byte("blocker"), 0444))

	err := a.installGitHooks(cfg)
	if err != nil {
		assert.Contains(t, err.Error(), "hook")
	}
}
