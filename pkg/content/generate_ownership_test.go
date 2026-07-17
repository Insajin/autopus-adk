package content

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsGeneratedTemplatePath(t *testing.T) {
	generated := []string{
		"codex/agents/executor.toml.tmpl",
		"gemini/agents/executor.md.tmpl",
		"codex/skills/testing-strategy.md.tmpl",
		"gemini/skills/testing-strategy/SKILL.md.tmpl",
		"claude/workflows/route_a.workflow.js.tmpl",
	}
	for _, rel := range generated {
		assert.True(t, IsGeneratedTemplatePath(rel), rel)
	}

	static := []string{
		"codex/prompts/auto-fix.md.tmpl",
		"codex/skills/auto-fix.md.tmpl",
		"gemini/skills/auto-fix/SKILL.md.tmpl",
		"gemini/rules/autopus/doc-storage.md.tmpl",
		"shared/autopus.yaml.tmpl",
		"../templates/codex/agents/executor.toml.tmpl",
	}
	for _, rel := range static {
		assert.False(t, IsGeneratedTemplatePath(rel), rel)
	}
}
