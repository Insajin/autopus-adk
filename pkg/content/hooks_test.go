// Package content_test는 훅 설정 생성 패키지의 테스트이다.
package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/content"
)

func TestGenerateHookConfigs_WithHooks(t *testing.T) {
	t.Parallel()

	cfg := config.HooksConf{
		PreCommitArch:  true,
		PreCommitLore:  true,
		ReactCIFailure: false,
		ReactReview:    false,
	}

	hooks, gitHooks, err := content.GenerateHookConfigs(cfg, "claude", true)
	require.NoError(t, err)
	// CLI hooks: only arch generates PreToolUse; lore uses git commit-msg hook only.
	assert.NotEmpty(t, hooks)
	assert.Len(t, hooks, 1, "only arch check should be a CLI hook")
	assert.Equal(t, "PreToolUse", hooks[0].Event)
	assert.Contains(t, hooks[0].Command, "--arch")
	// Lore is NOT a CLI hook — it runs via git commit-msg hook.
	for _, h := range hooks {
		assert.NotContains(t, h.Command, "--lore", "lore should not be a CLI hook")
	}
	assert.Empty(t, gitHooks)
}

func TestGenerateHookConfigs_WithoutHooks(t *testing.T) {
	t.Parallel()

	cfg := config.HooksConf{
		PreCommitArch: true,
		PreCommitLore: true,
	}

	hooks, gitHooks, err := content.GenerateHookConfigs(cfg, "codex", false)
	require.NoError(t, err)
	// CLI hooks not supported — git hook scripts returned.
	assert.Empty(t, hooks)
	assert.NotEmpty(t, gitHooks)
	// pre-commit (arch --staged) + commit-msg (lore --message) both present.
	var paths []string
	for _, g := range gitHooks {
		paths = append(paths, g.Path)
	}
	assert.Contains(t, paths, ".git/hooks/pre-commit")
	assert.Contains(t, paths, ".git/hooks/commit-msg")
}

func TestGenerateHookConfigs_AllDisabled(t *testing.T) {
	t.Parallel()

	cfg := config.HooksConf{
		PreCommitArch:  false,
		PreCommitLore:  false,
		ReactCIFailure: false,
		ReactReview:    false,
	}

	hooks, gitHooks, err := content.GenerateHookConfigs(cfg, "claude", true)
	require.NoError(t, err)
	assert.Empty(t, hooks)
	assert.Empty(t, gitHooks)
}

func TestGenerateHookConfigs_DeduplicatesReactHooks(t *testing.T) {
	t.Parallel()

	cfg := config.HooksConf{
		ReactCIFailure: true,
		ReactReview:    true,
	}

	hooks, _, err := content.GenerateHookConfigs(cfg, "claude", true)
	require.NoError(t, err)
	require.Len(t, hooks, 1)
	assert.Equal(t, "auto react check --quiet", hooks[0].Command)
}

func TestGitHookScript_Content(t *testing.T) {
	t.Parallel()

	cfg := config.HooksConf{
		PreCommitArch: true,
	}

	_, gitHooks, err := content.GenerateHookConfigs(cfg, "gemini", false)
	require.NoError(t, err)
	require.NotEmpty(t, gitHooks)

	// Script uses --staged to only check staged files.
	assert.Contains(t, gitHooks[0].Content, "auto check --arch --quiet --staged")
}

func TestDetectPermissions_DefaultOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	perms := content.DetectPermissions(dir, config.PermissionsConf{})

	assert.NotNil(t, perms)
	assert.Contains(t, perms.Allow, "Bash(auto *)")
	assert.Contains(t, perms.Allow, "Bash(git *)")
	assert.Contains(t, perms.Allow, "WebSearch")
	assert.NotContains(t, perms.Allow, "Bash(go test:*)")
	assert.NotContains(t, perms.Allow, "Bash(npm *)")
}

func TestDetectPermissions_GoProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644))

	perms := content.DetectPermissions(dir, config.PermissionsConf{})

	assert.Contains(t, perms.Allow, "Bash(go test:*)")
	assert.Contains(t, perms.Allow, "Bash(go build:*)")
	assert.Contains(t, perms.Allow, "Bash(golangci-lint:*)")
	assert.NotContains(t, perms.Allow, "Bash(npm *)")
}

func TestDetectPermissions_NodeProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644))

	perms := content.DetectPermissions(dir, config.PermissionsConf{})

	assert.Contains(t, perms.Allow, "Bash(npm *)")
	assert.Contains(t, perms.Allow, "Bash(npx *)")
	assert.NotContains(t, perms.Allow, "Bash(go test:*)")
}

func TestDetectPermissions_ExtraPerms(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	extra := config.PermissionsConf{
		ExtraAllow: []string{"Bash(cargo build:*)"},
		ExtraDeny:  []string{"Bash(rm -rf:*)"},
	}

	perms := content.DetectPermissions(dir, extra)

	assert.Contains(t, perms.Allow, "Bash(cargo build:*)")
	assert.Contains(t, perms.Deny, "Bash(rm -rf:*)")
	assert.Contains(t, perms.Allow, "Bash(auto *)")
}
