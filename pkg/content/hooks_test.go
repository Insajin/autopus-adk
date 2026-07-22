// Package content_test는 훅 설정 생성 패키지의 테스트이다.
package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
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
	// CLI hooks: arch check (PreToolUse) + completion Stop hook.
	assert.NotEmpty(t, hooks)
	// Lore is NOT a CLI hook — it runs via git commit-msg hook.
	for _, h := range hooks {
		assert.NotContains(t, h.Command, "--lore", "lore should not be a CLI hook")
	}
	var archHook *adapter.HookConfig
	for i := range hooks {
		if hooks[i].Event == "PreToolUse" {
			archHook = &hooks[i]
		}
	}
	require.NotNil(t, archHook, "expected a PreToolUse arch hook")
	assert.Contains(t, archHook.Command, "--hygiene")
	assert.Contains(t, archHook.Command, "--arch")
	assert.Contains(t, archHook.Command, "--staged")
	assert.Contains(t, archHook.Command, "--warn-only")
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
	// pre-commit (hygiene + arch --staged) + commit-msg (lore --message) both present.
	var paths []string
	for _, g := range gitHooks {
		paths = append(paths, g.Path)
	}
	assert.Contains(t, paths, ".git/hooks/pre-commit")
	assert.Contains(t, paths, ".git/hooks/commit-msg")
	for _, g := range gitHooks {
		if g.Path == ".git/hooks/commit-msg" {
			assert.Contains(t, g.Content, "auto check --lore --quiet --message")
			assert.Contains(t, g.Content, "auto lore validate \"$1\"")
		}
	}
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
	// All HooksConf fields disabled — the unconditional orchestra hook-IPC hooks
	// remain: the completion Stop hook and the SessionStart ready hook (SPEC-ORCH-022).
	require.Len(t, hooks, 2, "completion + session-start ready hooks should be present when all opts disabled")
	events := map[string]string{}
	for _, h := range hooks {
		events[h.Event] = h.Command
	}
	assert.Equal(t, `"${CLAUDE_PROJECT_DIR:-.}"/.claude/hooks/autopus/hook-claude-stop.sh`, events["Stop"])
	assert.Equal(t, `"${CLAUDE_PROJECT_DIR:-.}"/.claude/hooks/autopus/hook-claude-sessionstart.sh`, events["SessionStart"])
	assert.Empty(t, gitHooks)
}

// TestGenerateHookConfigs_GeminiTranslatesEventNames verifies that the legacy
// Gemini CLI surface receives BeforeTool/AfterTool instead of Claude Code's
// PreToolUse/PostToolUse, which that CLI rejected as
// "Invalid hook event name".
func TestGenerateHookConfigs_GeminiTranslatesEventNames(t *testing.T) {
	t.Parallel()

	cfg := config.HooksConf{
		PreCommitArch:  true,
		ReactCIFailure: true,
	}

	hooks, _, err := content.GenerateHookConfigs(cfg, "gemini", true)
	require.NoError(t, err)

	eventNames := make([]string, len(hooks))
	for i, h := range hooks {
		eventNames[i] = h.Event
	}

	assert.Contains(t, eventNames, "BeforeTool", "PreToolUse must be translated to BeforeTool for gemini")
	assert.Contains(t, eventNames, "AfterTool", "PostToolUse must be translated to AfterTool for gemini")
	assert.NotContains(t, eventNames, "PreToolUse", "gemini hooks must not use Claude Code event names")
	assert.NotContains(t, eventNames, "PostToolUse", "gemini hooks must not use Claude Code event names")
}

func TestGenerateHookConfigs_AntigravityKeepsOfficialEventNames(t *testing.T) {
	t.Parallel()

	cfg := config.HooksConf{
		PreCommitArch:  true,
		ReactCIFailure: true,
	}

	hooks, _, err := content.GenerateHookConfigs(cfg, "antigravity-cli", true)
	require.NoError(t, err)

	eventNames := make([]string, len(hooks))
	for i, h := range hooks {
		eventNames[i] = h.Event
	}

	assert.Contains(t, eventNames, "PreToolUse")
	assert.Contains(t, eventNames, "PostToolUse")
	assert.NotContains(t, eventNames, "BeforeTool")
	assert.NotContains(t, eventNames, "AfterTool")
	// Stop is the completion hook in the Antigravity lifecycle.
	assert.Contains(t, eventNames, "Stop")

	// Tool-use hooks must be wrapped for Antigravity JSON stdout protocol;
	// the completion Stop hook is a plain command (not tool-use).
	for _, h := range hooks {
		if h.Event == "Stop" {
			// Completion hook: plain command, no run_command matcher.
			continue
		}
		assert.Equal(t, "run_command", h.Matcher, "Antigravity tool-use hooks must match official tool names")
		assert.Contains(t, h.Command, "sh -c", "Antigravity tool-use hooks must wrap commands for JSON stdout")
		assert.Contains(t, h.Command, ">&2", "Antigravity tool-use hooks should keep command output off stdout")
		if h.Event == "PreToolUse" {
			assert.Contains(t, h.Command, `decision`, "PreToolUse must emit a decision JSON object")
		}
		if h.Event == "PostToolUse" {
			assert.Contains(t, h.Command, `{}`, "PostToolUse must emit an empty JSON object")
		}
	}
}

// TestGenerateHookConfigs_ClaudeKeepsEventNames verifies claude-code still
// receives PreToolUse/PostToolUse (its native event names).
func TestGenerateHookConfigs_ClaudeKeepsEventNames(t *testing.T) {
	t.Parallel()

	cfg := config.HooksConf{
		PreCommitArch:  true,
		ReactCIFailure: true,
	}

	hooks, _, err := content.GenerateHookConfigs(cfg, "claude", true)
	require.NoError(t, err)

	eventNames := make([]string, len(hooks))
	for i, h := range hooks {
		eventNames[i] = h.Event
	}

	assert.Contains(t, eventNames, "PreToolUse")
	assert.Contains(t, eventNames, "PostToolUse")
	assert.NotContains(t, eventNames, "BeforeTool")
	assert.NotContains(t, eventNames, "AfterTool")
}

func TestGenerateHookConfigs_DeduplicatesReactHooks(t *testing.T) {
	t.Parallel()

	cfg := config.HooksConf{
		ReactCIFailure: true,
		ReactReview:    true,
	}

	hooks, _, err := content.GenerateHookConfigs(cfg, "claude", true)
	require.NoError(t, err)
	// ReactCIFailure and ReactReview both enabled — dedup keeps only one PostToolUse react hook,
	// plus the unconditional completion Stop hook and the SessionStart ready hook (SPEC-ORCH-022).
	require.Len(t, hooks, 3, "expected one deduped react hook plus the completion Stop and SessionStart ready hooks")
	var reactHook *adapter.HookConfig
	for i := range hooks {
		if hooks[i].Event == "PostToolUse" {
			reactHook = &hooks[i]
		}
	}
	require.NotNil(t, reactHook, "expected a PostToolUse react hook")
	assert.Equal(t, "auto react check --quiet", reactHook.Command)
}

func TestGenerateProjectHookConfigs_ClaudeTaskCreatedEnabled(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultFullConfig("demo")
	cfg.Hooks = config.HooksConf{}
	cfg.Features.CC21 = config.CC21FeaturesConf{
		Enabled:                 true,
		EffortEnabled:           true,
		MonitorEnabled:          true,
		TaskCreatedEnabled:      true,
		InitialPromptEnabled:    true,
		TaskCreatedMode:         "warn",
		MonitorPatternTimeoutMS: 30000,
	}

	hooks, gitHooks, err := content.GenerateProjectHookConfigs(cfg, "claude-code", true)
	require.NoError(t, err)
	// Expect: completion Stop hook + SessionStart ready hook (SPEC-ORCH-022) + TaskCreated hook.
	require.Len(t, hooks, 3)
	assert.Empty(t, gitHooks)
	var taskCreatedHook *adapter.HookConfig
	for i := range hooks {
		if hooks[i].Event == "TaskCreated" {
			taskCreatedHook = &hooks[i]
		}
	}
	require.NotNil(t, taskCreatedHook, "expected a TaskCreated hook")
	assert.Equal(t, ".claude/hooks/task-created-validate.sh", taskCreatedHook.Command)
	assert.Equal(t, "warn", taskCreatedHook.Env["AUTOPUS_TASKCREATED_DEFAULT_MODE"])
}

func TestGenerateProjectHookConfigs_TaskCreatedDisabledOutsideClaude(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultFullConfig("demo")
	cfg.Hooks = config.HooksConf{}
	cfg.Features.CC21 = config.CC21FeaturesConf{
		Enabled:            true,
		TaskCreatedEnabled: true,
		TaskCreatedMode:    "enforce",
	}

	hooks, gitHooks, err := content.GenerateProjectHookConfigs(cfg, "codex", true)
	require.NoError(t, err)
	// TaskCreated is disabled outside claude; Codex still receives the completion
	// and readiness hooks required by pane IPC.
	require.Len(t, hooks, 2, "completion Stop and SessionStart hooks expected for codex")
	assert.ElementsMatch(t, []string{"Stop", "SessionStart"}, eventNames(hooks))
	assert.Empty(t, gitHooks)
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
	assert.Contains(t, gitHooks[0].Content, "auto check --hygiene --arch --quiet --staged")
}
