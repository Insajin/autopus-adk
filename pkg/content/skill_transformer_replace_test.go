package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestReplacePlatformReferences_AgentCallCodex(t *testing.T) {
	t.Parallel()

	// S3: Agent() → spawn_agent for codex
	input := `Some text.
Agent(subagent_type="executor", task="implement feature")
More text.`

	result := content.ReplacePlatformReferences(input, "codex")
	assert.Contains(t, result, `spawn_agent executor --task "implement feature"`)
	assert.NotContains(t, result, "Agent(subagent_type=")
	assert.Contains(t, result, "Some text.")
	assert.Contains(t, result, "More text.")
}

func TestReplacePlatformReferences_AgentCallGemini(t *testing.T) {
	t.Parallel()

	input := `Agent(subagent_type="executor", task="implement feature")`
	result := content.ReplacePlatformReferences(input, "gemini")
	assert.Contains(t, result, "@executor implement feature")
	assert.NotContains(t, result, "Agent(subagent_type=")
}

func TestReplacePlatformReferences_AgentCallNoTask(t *testing.T) {
	t.Parallel()

	input := `Agent(subagent_type="reviewer")`
	result := content.ReplacePlatformReferences(input, "codex")
	assert.Contains(t, result, "spawn_agent reviewer")
	assert.NotContains(t, result, "Agent(subagent_type=")
}

func TestReplacePlatformReferences_AgentCallPromptParam(t *testing.T) {
	t.Parallel()

	input := `Agent(subagent_type="executor", prompt="Implement T1")`
	result := content.ReplacePlatformReferences(input, "codex")
	assert.Contains(t, result, `spawn_agent executor --task "Implement T1"`)
}

func TestReplacePlatformReferences_MCPResolve(t *testing.T) {
	t.Parallel()

	// S4: mcp → WebSearch
	input := `Call mcp__context7__resolve-library-id(libraryName)
Call mcp__context7__query-docs(libraryId)
Normal line here.`

	result := content.ReplacePlatformReferences(input, "codex")
	assert.Contains(t, result, `WebSearch "libraryName docs"`)
	assert.Contains(t, result, `WebSearch "libraryId docs"`)
	assert.NotContains(t, result, "mcp__context7__")
	assert.Contains(t, result, "Normal line here.")
}

func TestReplacePlatformReferences_MCPGemini(t *testing.T) {
	t.Parallel()

	input := `mcp__context7__resolve-library-id("cobra")`
	result := content.ReplacePlatformReferences(input, "gemini")
	assert.Contains(t, result, `WebSearch "cobra docs"`)
	assert.NotContains(t, result, "mcp__")
}

func TestReplacePlatformReferences_PathsCodex(t *testing.T) {
	t.Parallel()

	// S5: .claude/ → .codex/ for codex
	input := `See .claude/agents/autopus/ for details.
Regular line.
Ref: .claude/rules/autopus/context7-docs.md
Skills at .claude/skills/autopus/tdd.md`

	result := content.ReplacePlatformReferences(input, "codex")
	assert.Contains(t, result, ".codex/agents/")
	assert.Contains(t, result, ".codex/rules/")
	assert.Contains(t, result, ".codex/skills/")
	assert.NotContains(t, result, ".claude/")
	assert.Contains(t, result, "Regular line.")
}

func TestReplacePlatformReferences_PathsGemini(t *testing.T) {
	t.Parallel()

	input := `See .claude/agents/autopus/ for details.`
	result := content.ReplacePlatformReferences(input, "gemini")
	assert.Contains(t, result, ".gemini/agents/")
	assert.NotContains(t, result, ".claude/")
}

func TestReplacePlatformReferences_ClaudeUnchanged(t *testing.T) {
	t.Parallel()

	// S8: backward compat — Claude returns unchanged
	input := `Call mcp__context7__resolve-library-id(libraryName)
Agent(subagent_type="executor")
.claude/skills/test.md`

	result := content.ReplacePlatformReferences(input, "claude")
	assert.Equal(t, input, result)
}

func TestReplacePlatformReferences_ClaudeCodeUnchanged(t *testing.T) {
	t.Parallel()

	input := `Agent(subagent_type="executor")
.claude/skills/test.md`

	result := content.ReplacePlatformReferences(input, "claude-code")
	assert.Equal(t, input, result)
}

func TestReplacePlatformReferences_WorktreeIsolation(t *testing.T) {
	t.Parallel()

	input := `Use isolation: "worktree" for parallel execution.`
	result := content.ReplacePlatformReferences(input, "codex")
	assert.Contains(t, result, "auto pipeline worktree")
	assert.NotContains(t, result, `isolation: "worktree"`)
}

func TestReplacePlatformReferences_TodoWrite(t *testing.T) {
	t.Parallel()

	input := `Use TodoWrite to track tasks.
Normal line.`

	result := content.ReplacePlatformReferences(input, "codex")
	assert.Contains(t, result, "// TodoWrite is not available on this platform")
	assert.Contains(t, result, "Normal line.")
}

func TestReplacePlatformReferences_EmptyBody(t *testing.T) {
	t.Parallel()

	result := content.ReplacePlatformReferences("", "codex")
	assert.Equal(t, "", result)
}

func TestReplacePlatformReferences_NoReferences(t *testing.T) {
	t.Parallel()

	input := `This is plain content.
No platform references here.`

	result := content.ReplacePlatformReferences(input, "codex")
	assert.Equal(t, input, result)
}

func TestReplacePlatformReferences_MultiPattern(t *testing.T) {
	t.Parallel()

	// Line with both .claude/ path and Agent() call
	input := `See .claude/agents/ and use Agent(subagent_type="tester", task="run tests")`
	result := content.ReplacePlatformReferences(input, "codex")
	assert.Contains(t, result, ".codex/agents/")
	assert.Contains(t, result, `spawn_agent tester --task "run tests"`)
	assert.NotContains(t, result, ".claude/")
	assert.NotContains(t, result, "Agent(subagent_type=")
}

func TestReplacePlatformReferences_MixedContent(t *testing.T) {
	t.Parallel()

	input := `# Title
Normal line 1.
Agent(subagent_type="executor", task="build")
Normal line 2.
mcp__context7__resolve-library-id(cobra)
Normal line 3.
See .claude/skills/tdd.md`

	result := content.ReplacePlatformReferences(input, "codex")
	assert.Contains(t, result, "# Title")
	assert.Contains(t, result, "Normal line 1.")
	assert.Contains(t, result, `spawn_agent executor --task "build"`)
	assert.Contains(t, result, "Normal line 2.")
	assert.Contains(t, result, `WebSearch "cobra docs"`)
	assert.Contains(t, result, "Normal line 3.")
	assert.Contains(t, result, ".codex/skills/tdd.md")
}

func TestNormalizeAgentReferences_BrandingPaths(t *testing.T) {
	t.Parallel()

	input := "- **브랜딩**: `content/rules/branding.md` 준수\n- **출력 포맷**: A3 (Agent Result Format) — `branding-formats.md.tmpl` 참조"

	assert.Contains(t, content.NormalizeAgentReferences(input, "claude-code"), ".claude/rules/autopus/branding.md")
	assert.Contains(t, content.NormalizeAgentReferences(input, "codex"), ".codex/rules/autopus/branding.md")
	assert.Contains(t, content.NormalizeAgentReferences(input, "gemini"), ".gemini/rules/autopus/branding.md")
	assert.Contains(t, content.NormalizeAgentReferences(input, "opencode"), ".opencode/rules/autopus/branding.md")
	assert.NotContains(t, content.NormalizeAgentReferences(input, "codex"), "content/rules/branding.md")
	assert.Contains(t, content.NormalizeAgentReferences(input, "codex"), "templates/shared/branding-formats.md.tmpl")
}
