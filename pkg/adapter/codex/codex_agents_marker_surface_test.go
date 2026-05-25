package codex_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestCodexAndOpenCode_AGENTSMD_UsesSharedPlatformSection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := config.DefaultFullConfig("shared-project")
	cfg.Platforms = []string{"codex", "opencode"}

	codexAdapter := codex.NewWithRoot(dir)
	_, err := codexAdapter.Generate(context.Background(), cfg)
	require.NoError(t, err)

	opencodeAdapter := opencode.NewWithRoot(dir)
	_, err = opencodeAdapter.Generate(context.Background(), cfg)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "- **플랫폼**: codex, opencode")
	assert.Contains(t, content, "Codex Rules: .codex/rules/autopus/")
	assert.Contains(t, content, "OpenCode Rules: .opencode/rules/autopus/")
	assert.Contains(t, content, "**Codex**: 하네스 기본값은 spawn_agent(...) 기반 subagent-first 입니다.")
	assert.Contains(t, content, "**Codex /goal**: Codex goals feature를 사용합니다.")
	assert.Contains(t, content, "**Codex --team**: native multi_agent 도구(spawn_agent/send_input/wait_agent/close_agent) 기반 Lead/Builder/Guardian 팀 프로파일입니다.")
	assert.Contains(t, content, "**OpenCode**: 기본 실행 모델은 task(...) 기반 subagent-first 입니다.")
	assert.Contains(t, content, "## Core Guidelines")
	assert.Contains(t, content, "SPEC Markdown files under .autopus/specs/**")
	assert.Contains(t, content, "See .codex/rules/autopus/ for Codex rule definitions.")
	assert.Contains(t, content, "See .codex/skills/agent-pipeline.md for phase and gate contracts.")
	assert.Contains(t, content, "See .opencode/rules/autopus/ for OpenCode guidance.")
}
