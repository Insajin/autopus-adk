package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestTransformAgentForOpenCode_PermissionsAndBody(t *testing.T) {
	t.Parallel()

	result := content.TransformAgentForOpenCode(makeExecutorSource())

	assert.Contains(t, result, `description: "TDD/DDD implementation agent"`)
	assert.Contains(t, result, `mode: subagent`)
	assert.Contains(t, result, `steps: 50`)
	assert.Contains(t, result, `"*": deny`)
	assert.Contains(t, result, `"edit": allow`)
	assert.Contains(t, result, `"bash": allow`)
	assert.Contains(t, result, `"task": allow`)
	assert.Contains(t, result, `"skill": allow`)
	assert.Contains(t, result, `"question": allow`)
	assert.Contains(t, result, `Use the following Autopus skills when they fit the task`)
	assert.Contains(t, result, `task tool → subagent_type="tester", prompt="run tests"`)
	assert.Contains(t, result, `.opencode/rules/`)
	assert.Contains(t, result, `todowrite`)
}

func TestReplacePlatformReferences_OpenCodeMappings(t *testing.T) {
	t.Parallel()

	input := `Use Agent(subagent_type="reviewer", task="check diff")
Read @.claude/skills/autopus/worktree-isolation.md before continuing.
AskUserQuestion if requirements are unclear.
TodoWrite before implementation.`

	result := content.ReplacePlatformReferences(input, "opencode")

	assert.Contains(t, result, `task tool → subagent_type="reviewer", prompt="check diff"`)
	assert.Contains(t, result, `.agents/skills/worktree-isolation/SKILL.md`)
	assert.Contains(t, result, `question if requirements are unclear.`)
	assert.Contains(t, result, `todowrite before implementation.`)
}
