package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestSkillTransformer_IsCompatible_NoPlatforms(t *testing.T) {
	t.Parallel()

	// No platforms field = compatible with all platforms (R6)
	meta := content.SkillMeta{
		Name:      "planning",
		Platforms: nil,
	}
	assert.True(t, content.IsCompatible(meta, "codex"))
	assert.True(t, content.IsCompatible(meta, "gemini"))
	assert.True(t, content.IsCompatible(meta, "claude"))
}

func TestSkillTransformer_IsCompatible_ClaudeOnly(t *testing.T) {
	t.Parallel()

	meta := content.SkillMeta{
		Name:      "agent-pipeline",
		Platforms: []string{"claude"},
	}
	assert.False(t, content.IsCompatible(meta, "codex"))
	assert.False(t, content.IsCompatible(meta, "gemini"))
	assert.True(t, content.IsCompatible(meta, "claude"))
}

func TestSkillTransformer_IsCompatible_ClaudeAndCodex(t *testing.T) {
	t.Parallel()

	meta := content.SkillMeta{
		Name:      "tdd",
		Platforms: []string{"claude", "codex"},
	}
	assert.True(t, content.IsCompatible(meta, "codex"))
	assert.False(t, content.IsCompatible(meta, "gemini"))
	assert.True(t, content.IsCompatible(meta, "claude"))
}

func TestFilterPlatformReferences_MCP(t *testing.T) {
	t.Parallel()

	input := `Step 1:
Call mcp__context7__resolve-library-id(libraryName)
Call mcp__context7__query-docs(libraryId)
Normal line here.`

	result := content.FilterPlatformReferences(input, "codex")
	assert.NotContains(t, result, "mcp__context7__resolve-library-id")
	assert.NotContains(t, result, "mcp__context7__query-docs")
	assert.Contains(t, result, "Normal line here.")
}

func TestFilterPlatformReferences_AgentSubagent(t *testing.T) {
	t.Parallel()

	input := `Some text.
Agent(subagent_type="executor", prompt="Implement T1")
More text.`

	result := content.FilterPlatformReferences(input, "codex")
	assert.NotContains(t, result, `Agent(subagent_type=`)
	assert.Contains(t, result, "Some text.")
	assert.Contains(t, result, "More text.")
}

func TestFilterPlatformReferences_ClaudePaths(t *testing.T) {
	t.Parallel()

	input := `See .claude/skills/autopus/agent-teams.md for details.
Regular line.
Ref: .claude/rules/autopus/context7-docs.md`

	result := content.FilterPlatformReferences(input, "gemini")
	assert.NotContains(t, result, ".claude/")
	assert.Contains(t, result, "Regular line.")
}

func TestFilterPlatformReferences_PreservesForClaude(t *testing.T) {
	t.Parallel()

	input := `Call mcp__context7__resolve-library-id(libraryName)
Agent(subagent_type="executor")
.claude/skills/test.md`

	// Claude platform should preserve all references
	result := content.FilterPlatformReferences(input, "claude")
	assert.Equal(t, input, result)
}

func TestSkillTransformer_TransformForPlatform(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Skill with no platforms (compatible with all)
	writeSkillFile(t, dir, "tdd.md", `---
name: tdd
description: TDD skill
triggers:
  - tdd
category: methodology
---

# TDD Skill

Use Agent(subagent_type="executor") to run.
Call mcp__context7__resolve-library-id(name).
See .claude/skills/autopus/tdd.md.
Normal content here.`)

	// Claude-only skill
	writeSkillFile(t, dir, "pipeline.md", `---
name: pipeline
description: Pipeline skill
platforms:
  - claude
triggers:
  - pipeline
category: agentic
---

# Pipeline

Claude-only content.`)

	transformer, err := content.NewSkillTransformer(dir)
	require.NoError(t, err)

	// Transform for Codex
	skills, report, err := transformer.TransformForPlatform("codex")
	require.NoError(t, err)

	// Only tdd should be included (pipeline is claude-only)
	assert.Len(t, skills, 1)
	assert.Equal(t, "tdd", skills[0].Name)
	// ReplacePlatformReferences replaces instead of removing
	assert.NotContains(t, skills[0].Content, "mcp__context7__")
	assert.Contains(t, skills[0].Content, "WebSearch")
	assert.NotContains(t, skills[0].Content, "Agent(subagent_type=")
	assert.Contains(t, skills[0].Content, "spawn_agent executor")
	assert.NotContains(t, skills[0].Content, ".claude/")
	assert.Contains(t, skills[0].Content, ".codex/skills/")
	assert.Contains(t, skills[0].Content, "Normal content here.")

	// Report should list compatible and incompatible
	assert.Len(t, report.Compatible, 1)
	assert.Len(t, report.Incompatible, 1)
	assert.Equal(t, "pipeline", report.Incompatible[0])
}

func TestSkillTransformer_TransformForPlatform_Gemini(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeSkillFile(t, dir, "review.md", `---
name: review
description: Review skill
triggers:
  - review
category: quality
---

# Review

Normal review content.`)

	transformer, err := content.NewSkillTransformer(dir)
	require.NoError(t, err)

	skills, report, err := transformer.TransformForPlatform("gemini")
	require.NoError(t, err)

	assert.Len(t, skills, 1)
	assert.Equal(t, "review", skills[0].Name)
	assert.Contains(t, skills[0].Content, "Normal review content.")
	assert.Len(t, report.Compatible, 1)
	assert.Empty(t, report.Incompatible)
}

func TestSkillTransformer_TransformForPlatform_UnknownPlatform(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	transformer, err := content.NewSkillTransformer(dir)
	require.NoError(t, err)

	_, _, err = transformer.TransformForPlatform("unknown")
	assert.Error(t, err)
}

func TestSkillTransformer_EmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	transformer, err := content.NewSkillTransformer(dir)
	require.NoError(t, err)

	skills, report, err := transformer.TransformForPlatform("codex")
	require.NoError(t, err)
	assert.Empty(t, skills)
	assert.Empty(t, report.Compatible)
	assert.Empty(t, report.Incompatible)
}
