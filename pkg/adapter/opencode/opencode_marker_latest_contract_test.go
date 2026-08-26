package opencode

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestInjectMarkerSection_MixedModeAdvertisesLatestCodexNativeSurface(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultFullConfig("demo")
	cfg.Platforms = []string{"codex", "opencode"}

	section, err := NewWithRoot(t.TempDir()).injectMarkerSection(cfg)
	require.NoError(t, err)

	assert.Contains(t, section, ".codex/skills/codex-<name>/SKILL.md")
	assert.Contains(t, section, ".codex/skills/codex-agent-pipeline/SKILL.md")
	assert.Contains(t, section, "@auto")
	assert.Contains(t, section, "$codex-auto-<route>")
	assert.Contains(t, section, "spawn_agent, send_message, followup_task, wait_agent, interrupt_agent, list_agents")
	assert.Contains(t, section, "shared cwd/filesystem")
	assert.Contains(t, section, "fork_turns")
	assert.NotContains(t, section, ".codex/rules")
	assert.NotContains(t, section, ".codex/prompts")
	assert.NotContains(t, section, "send_input")
	assert.NotContains(t, section, "resume_agent")
	assert.NotContains(t, section, "close_agent")
	assert.NotContains(t, section, ".codex/skills/agent-pipeline.md")
}

func TestInjectMarkerSection_UpdatePreservesCodexDollarInvocation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	existing := "user\n" + markerBegin + "\nold\n" + markerEnd + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(existing), 0o644))
	cfg := config.DefaultFullConfig("demo")
	cfg.Platforms = []string{"codex", "opencode"}

	section, err := NewWithRoot(root).injectMarkerSection(cfg)

	require.NoError(t, err)
	assert.Contains(t, section, "$codex-auto-<route>")
	assert.NotContains(t, section, "/ -auto-<route>")
}
