package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/content"
)

// TestReplacePlatformReferences_GeminiCLI verifies gemini-cli platform normalization.
func TestReplacePlatformReferences_GeminiCLI(t *testing.T) {
	t.Parallel()

	input := `See .claude/rules/test.md and Agent(subagent_type="exec", task="run")`
	result := content.ReplacePlatformReferences(input, "gemini-cli")

	assert.Contains(t, result, ".gemini/rules/test.md")
	assert.NotContains(t, result, ".claude/")
	assert.Contains(t, result, "@exec run")
}

// TestReplacePlatformReferences_UnknownPlatformPassthrough verifies unknown platforms
// don't crash and Agent calls remain unchanged.
func TestReplacePlatformReferences_UnknownPlatform(t *testing.T) {
	t.Parallel()

	input := `Agent(subagent_type="test") and .claude/skills/test.md`
	result := content.ReplacePlatformReferences(input, "vscode")

	// Agent call is not transformed for unknown platform
	assert.Contains(t, result, "Agent(subagent_type=")
	// Paths are not transformed either (no mapping exists)
	assert.Contains(t, result, ".claude/skills/test.md")
}

// TestReplaceToolReferences_GeminiCLI verifies gemini-cli agent/path mapping.
func TestReplaceToolReferences_GeminiCLI(t *testing.T) {
	t.Parallel()

	body := `Agent(subagent_type="tester", task="verify") and .claude/agents/dir`
	result := content.ReplaceToolReferences(body, "gemini-cli")

	assert.Contains(t, result, "@tester verify")
	assert.Contains(t, result, ".gemini/agents/dir")
	assert.NotContains(t, result, ".claude/")
}

// TestReplacePlatformReferences_MCPWithQuotedArgs verifies mcp calls with quoted arguments.
func TestReplacePlatformReferences_MCPWithQuotedArgs(t *testing.T) {
	t.Parallel()

	input := `mcp__context7__resolve-library-id("cobra")
mcp__context7__query-docs("cobra", topic="routing")`
	result := content.ReplacePlatformReferences(input, "codex")

	assert.Contains(t, result, "Context7 MCP first")
	assert.Contains(t, result, `WebSearch "cobra docs"`)
	assert.Contains(t, result, `WebSearch "cobra routing docs"`)
	assert.NotContains(t, result, "mcp__context7__")
}

// TestReplacePlatformReferences_GenericMCPReference verifies non-context7 mcp references.
func TestReplacePlatformReferences_GenericMCPReference(t *testing.T) {
	t.Parallel()

	input := `Use mcp__some_other_tool for something.`
	result := content.ReplacePlatformReferences(input, "codex")

	assert.Contains(t, result, "WebSearch")
	assert.NotContains(t, result, "mcp__some_other_tool")
}

// TestReplacePlatformReferences_AllSubstitutionsAcrossLines verifies multiple replacement
// types across different lines (multi-pattern edge case).
func TestReplacePlatformReferences_AllSubstitutionsAcrossLines(t *testing.T) {
	t.Parallel()

	// TodoWrite replaces the entire line, so it must be on its own line.
	input := "Agent(subagent_type=\"e\", task=\"t\") .claude/rules/ mcp__context7__resolve-library-id(x)\nTodoWrite\nisolation: \"worktree\""
	result := content.ReplacePlatformReferences(input, "codex")

	assert.Contains(t, result, `spawn_agent e --task "t"`)
	assert.Contains(t, result, ".codex/rules/")
	assert.Contains(t, result, "Context7 MCP first")
	assert.Contains(t, result, `WebSearch "x docs"`)
	assert.Contains(t, result, "// TodoWrite is not available")
	assert.Contains(t, result, "auto pipeline worktree")
}

// TestReplacePlatformReferences_MultiplePathsOnSameLine verifies multiple .claude/ paths.
func TestReplacePlatformReferences_MultiplePathsOnSameLine(t *testing.T) {
	t.Parallel()

	input := `Copy from .claude/skills/ to .claude/agents/ or .claude/rules/`
	result := content.ReplacePlatformReferences(input, "codex")

	assert.Contains(t, result, ".codex/skills/")
	assert.Contains(t, result, ".codex/agents/")
	assert.Contains(t, result, ".codex/rules/")
	assert.NotContains(t, result, ".claude/")
}

// TestMapModel_DefaultPlatform verifies default/passthrough for unknown platform.
func TestMapModel_DefaultPlatform(t *testing.T) {
	t.Parallel()

	// Unknown platform returns model as-is
	assert.Equal(t, "sonnet", content.MapModel("sonnet", "vscode"))
	assert.Equal(t, "opus", content.MapModel("opus", ""))

	// Known platform, unknown model returns model as-is
	assert.Equal(t, "custom-model", content.MapModel("custom-model", "codex"))
}

// TestReplaceToolReferences_AgentNoTask verifies agent call without task parameter.
func TestReplaceToolReferences_AgentNoTask_Gemini(t *testing.T) {
	t.Parallel()

	body := `Use Agent(subagent_type="reviewer") for code review.`

	gemini := content.ReplaceToolReferences(body, "gemini")
	assert.Contains(t, gemini, "@reviewer")
	assert.NotContains(t, gemini, "Agent(subagent_type=")
}
