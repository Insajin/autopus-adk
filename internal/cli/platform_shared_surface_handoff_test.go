package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestPlatformAddCodexRefreshesOpenCodeOwnedSharedSurface(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.DefaultFullConfig("platform-add-codex-handoff")
	cfg.Platforms = []string{"opencode"}
	seedPlatformTransitionSurface(t, root, cfg)

	before := readPlatformTransitionFile(t, root, "AGENTS.md")
	require.NotContains(t, before, "Codex Native Skills")

	dirFlag := root
	cmd := newPlatformAddCmd(&dirFlag)
	cmd.SetArgs([]string{"codex"})
	require.NoError(t, cmd.Execute())

	after := readPlatformTransitionFile(t, root, "AGENTS.md")
	assert.Contains(t, after, "Codex Native Skills: .codex/skills/codex-<name>/SKILL.md")
	assert.Contains(t, after, "OpenCode Rules: .opencode/rules/autopus/")
	assert.Contains(t, after, "Codex V2")
	assert.Contains(t, after, "OpenCode Invocation")
}

func TestPlatformAddOpenCodeRelinquishesPreviousCodexRootClaim(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := config.DefaultFullConfig("platform-add-opencode-handoff")
	cfg.Platforms = []string{"codex"}
	seedPlatformTransitionSurface(t, root, cfg)
	before, err := adapter.LoadManifest(root, "codex")
	require.NoError(t, err)
	require.Contains(t, before.Files, "AGENTS.md")

	dirFlag := root
	cmd := newPlatformAddCmd(&dirFlag)
	cmd.SetArgs([]string{"opencode"})
	require.NoError(t, cmd.Execute())

	codexManifest, err := adapter.LoadManifest(root, "codex")
	require.NoError(t, err)
	opencodeManifest, err := adapter.LoadManifest(root, "opencode")
	require.NoError(t, err)
	assert.NotContains(t, codexManifest.Files, "AGENTS.md")
	assert.Contains(t, opencodeManifest.Files, "AGENTS.md")
	agents := readPlatformTransitionFile(t, root, "AGENTS.md")
	assert.Contains(t, agents, "Codex Native Skills")
	assert.Contains(t, agents, "OpenCode Invocation")
}

func TestPlatformAddOpenCodeOwnerFailureRollsBackBothPlatforms(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := config.DefaultFullConfig("platform-add-opencode-rollback")
	cfg.Platforms = []string{"codex"}
	seedPlatformTransitionSurface(t, root, cfg)
	configPath := filepath.Join(root, ".codex", "config.toml")
	require.NoError(t, os.Remove(configPath))
	require.NoError(t, os.MkdirAll(configPath, 0o755))
	agentsBefore := readPlatformTransitionFile(t, root, "AGENTS.md")
	configBefore := readPlatformTransitionFile(t, root, "autopus.yaml")
	manifestBefore := readPlatformTransitionFile(t, root, ".autopus", "codex-manifest.json")

	dirFlag := root
	cmd := newPlatformAddCmd(&dirFlag)
	cmd.SetArgs([]string{"opencode"})
	require.Error(t, cmd.Execute())

	assert.Equal(t, agentsBefore, readPlatformTransitionFile(t, root, "AGENTS.md"))
	assert.Equal(t, configBefore, readPlatformTransitionFile(t, root, "autopus.yaml"))
	assert.Equal(t, manifestBefore, readPlatformTransitionFile(t, root, ".autopus", "codex-manifest.json"))
	assert.NoFileExists(t, filepath.Join(root, ".autopus", "opencode-manifest.json"))
	assert.DirExists(t, configPath)
}

func TestPlatformRemoveOpenCodeRegeneratesCodexOwnedAgents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.DefaultFullConfig("platform-remove-opencode-handoff")
	cfg.Platforms = []string{"codex", "opencode"}
	seedPlatformTransitionSurface(t, root, cfg)

	dirFlag := root
	cmd := newPlatformRemoveCmd(&dirFlag)
	cmd.SetArgs([]string{"opencode"})
	require.NoError(t, cmd.Execute())

	after := readPlatformTransitionFile(t, root, "AGENTS.md")
	assert.Contains(t, after, "Codex Invocation")
	assert.Contains(t, after, "Codex V2")
	assert.Contains(t, after, "Codex Shared Workspace")
	assert.NotContains(t, after, "OpenCode Invocation")
	assert.NotContains(t, after, "OpenCode Rules: .opencode/rules/autopus/")
}

func TestPlatformRemoveCodexRefreshesOpenCodeOnlySurface(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.DefaultFullConfig("platform-remove-codex-handoff")
	cfg.Platforms = []string{"codex", "opencode"}
	seedPlatformTransitionSurface(t, root, cfg)

	dirFlag := root
	cmd := newPlatformRemoveCmd(&dirFlag)
	cmd.SetArgs([]string{"codex"})
	require.NoError(t, cmd.Execute())

	after := readPlatformTransitionFile(t, root, "AGENTS.md")
	assert.Contains(t, after, "OpenCode Invocation")
	assert.Contains(t, after, "task(...) 기반 subagent-first")
	assert.NotContains(t, after, "Codex Native Skills")
	assert.NotContains(t, after, "Codex V2")
	assert.NotContains(t, after, ".codex/skills/")

	autoSkill := readPlatformTransitionFile(t, root, ".agents", "skills", "auto", "SKILL.md")
	assert.NotContains(t, autoSkill, "[OpenCode-only]")
}

func seedPlatformTransitionSurface(t *testing.T, root string, cfg *config.HarnessConfig) {
	t.Helper()
	require.NoError(t, config.Save(root, cfg))

	for _, platform := range []string{"codex", "opencode"} {
		if !containsPlatform(cfg.Platforms, platform) {
			continue
		}
		descriptor, ok := lookupPlatformDescriptor(platform)
		require.True(t, ok)
		_, err := descriptor.Generate(context.Background(), root, cfg)
		require.NoError(t, err)
	}
}

func readPlatformTransitionFile(t *testing.T, root string, elements ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{root}, elements...)...))
	require.NoError(t, err)
	return string(data)
}
