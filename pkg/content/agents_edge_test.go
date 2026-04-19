package content_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestConvertAgentCodex_WithModelTierAndSkills(t *testing.T) {
	t.Parallel()

	agent := content.AgentDefinition{
		Name:      "executor",
		Role:      "Implementation agent",
		ModelTier: "sonnet",
		Skills:    []string{"tdd", "ddd"},
	}

	result, err := content.ConvertAgentToPlatform(agent, "codex")
	require.NoError(t, err)

	assert.Contains(t, result, "## Agent: executor")
	assert.Contains(t, result, "**Role:** Implementation agent")
	assert.Contains(t, result, "**Model Tier:** sonnet")
	assert.Contains(t, result, "**Skills:** tdd, ddd")
}

func TestConvertAgentCodex_WithInstructions(t *testing.T) {
	t.Parallel()

	agent := content.AgentDefinition{
		Name:         "planner",
		Role:         "Planning agent",
		Instructions: "Custom instruction body here.",
	}

	result, err := content.ConvertAgentToPlatform(agent, "codex")
	require.NoError(t, err)

	assert.Contains(t, result, "## Agent: planner")
	assert.Contains(t, result, "Custom instruction body here.")
}

func TestConvertAgentCodex_MinimalNoOptionalFields(t *testing.T) {
	t.Parallel()

	agent := content.AgentDefinition{
		Name: "bare",
		Role: "Bare agent",
	}

	result, err := content.ConvertAgentToPlatform(agent, "codex")
	require.NoError(t, err)

	assert.Contains(t, result, "## Agent: bare")
	assert.Contains(t, result, "**Role:** Bare agent")
	assert.NotContains(t, result, "**Model Tier:**")
	assert.NotContains(t, result, "**Skills:**")
}

func TestConvertAgentGemini_WithTriggersAndInstructions(t *testing.T) {
	t.Parallel()

	agent := content.AgentDefinition{
		Name:         "debugger",
		Role:         "Debug agent",
		Triggers:     []string{"debug", "fix"},
		Instructions: "Debug instruction body.",
	}

	result, err := content.ConvertAgentToPlatform(agent, "gemini")
	require.NoError(t, err)

	assert.Contains(t, result, "name: auto-agent-debugger")
	assert.Contains(t, result, "description: Debug agent")
	assert.Contains(t, result, "triggers:")
	assert.Contains(t, result, "  - debug")
	assert.Contains(t, result, "  - fix")
	assert.Contains(t, result, "# auto-agent-debugger")
	assert.Contains(t, result, "Debug instruction body.")
}

func TestConvertAgentGemini_NoTriggersNoInstructions(t *testing.T) {
	t.Parallel()

	agent := content.AgentDefinition{
		Name: "minimal",
		Role: "Minimal agent",
	}

	result, err := content.ConvertAgentToPlatform(agent, "gemini")
	require.NoError(t, err)

	assert.Contains(t, result, "name: auto-agent-minimal")
	assert.NotContains(t, result, "triggers:")
	// Without Instructions, falls back to Role text
	assert.Contains(t, result, "Minimal agent\n")
}

func TestConvertAgentClaude_WithInstructions(t *testing.T) {
	t.Parallel()

	agent := content.AgentDefinition{
		Name:         "tester",
		Role:         "Test agent",
		Instructions: "# Tester\n\nRun all tests.",
	}

	result, err := content.ConvertAgentToPlatform(agent, "claude")
	require.NoError(t, err)

	assert.Contains(t, result, "# Tester")
	assert.Contains(t, result, "Run all tests.")
	// Should NOT contain the fallback format
	assert.NotContains(t, result, "# tester\n\nTest agent")
}

func TestConvertAgentClaude_NoInstructionsFallback(t *testing.T) {
	t.Parallel()

	agent := content.AgentDefinition{
		Name: "planner",
		Role: "Planning agent",
	}

	result, err := content.ConvertAgentToPlatform(agent, "claude")
	require.NoError(t, err)

	// Without Instructions, falls back to "# Name\n\nRole"
	assert.Contains(t, result, "# planner\n\nPlanning agent")
}

func TestConvertAgentToPlatform_ClaudeCode(t *testing.T) {
	t.Parallel()

	agent := content.AgentDefinition{
		Name:      "executor",
		Role:      "Exec agent",
		ModelTier: "sonnet",
	}

	result, err := content.ConvertAgentToPlatform(agent, "claude-code")
	require.NoError(t, err)
	// claude-code shares the same path as claude
	assert.Contains(t, result, "name: executor")
	assert.Contains(t, result, "model_tier: sonnet")
}

func TestConvertAgentToPlatform_GeminiCLI(t *testing.T) {
	t.Parallel()

	agent := content.AgentDefinition{
		Name: "reviewer",
		Role: "Review agent",
	}

	result, err := content.ConvertAgentToPlatform(agent, "gemini-cli")
	require.NoError(t, err)
	assert.Contains(t, result, "name: auto-agent-reviewer")
}

func TestLoadAgentsFromFS_InvalidYAML(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"agents/bad.md": &fstest.MapFile{
			Data: []byte("---\nname: [invalid yaml\n---\n\nbody"),
		},
	}

	_, err := content.LoadAgentsFromFS(fsys, "agents")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad.md")
}

func TestLoadAgentsFromFS_SkipsNonMD(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"agents/readme.txt": &fstest.MapFile{Data: []byte("ignore me")},
		"agents/notes.json": &fstest.MapFile{Data: []byte("{}")},
		"agents/valid.md": &fstest.MapFile{
			Data: []byte("---\nname: valid\nrole: test\n---\n\nbody"),
		},
	}

	agents, err := content.LoadAgentsFromFS(fsys, "agents")
	require.NoError(t, err)
	assert.Len(t, agents, 1)
	assert.Equal(t, "valid", agents[0].Name)
}
