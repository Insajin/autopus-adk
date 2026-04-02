package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleSettings() *PlatformSettings {
	return &PlatformSettings{
		Agents: []AgentDef{
			{Name: "executor", Description: "Implements code", Model: "opus"},
			{Name: "tester", Description: "Writes tests", Model: "sonnet"},
		},
		Rules: []RuleDef{
			{Name: "file-size-limit", Category: "structure", Content: "Max 300 lines"},
			{Name: "lore-commit", Category: "workflow", Content: "Use lore format"},
		},
		Skills: []SkillDef{
			{Name: "auto-plan", Description: "Planning skill", Category: "workflow"},
		},
		Hooks: []HookDef{
			{Event: "Stop", Matcher: "", Command: "echo done", Timeout: 5},
		},
	}
}

func TestClaudeJSON_RoundTrip(t *testing.T) {
	original := sampleSettings()

	data, err := original.ToClaudeJSON()
	require.NoError(t, err)
	require.NotEmpty(t, data)

	parsed, err := ParseClaudeJSON(data)
	require.NoError(t, err)

	assert.Equal(t, original.Agents, parsed.Agents)
	assert.Equal(t, original.Rules, parsed.Rules)
	assert.Equal(t, original.Skills, parsed.Skills)
	require.Len(t, parsed.Hooks, len(original.Hooks))
	assert.Equal(t, original.Hooks[0].Event, parsed.Hooks[0].Event)
	assert.Equal(t, original.Hooks[0].Command, parsed.Hooks[0].Command)
	assert.Equal(t, original.Hooks[0].Timeout, parsed.Hooks[0].Timeout)
}

func TestGeminiJSON_RoundTrip(t *testing.T) {
	original := sampleSettings()

	data, err := original.ToGeminiJSON()
	require.NoError(t, err)
	require.NotEmpty(t, data)

	parsed, err := ParseGeminiJSON(data)
	require.NoError(t, err)

	// Agents round-trip fully
	assert.Equal(t, original.Agents, parsed.Agents)

	// Rules/Skills: Gemini only stores names, so content/category are lost
	require.Len(t, parsed.Rules, len(original.Rules))
	assert.Equal(t, original.Rules[0].Name, parsed.Rules[0].Name)
	assert.Equal(t, original.Rules[1].Name, parsed.Rules[1].Name)

	require.Len(t, parsed.Skills, len(original.Skills))
	assert.Equal(t, original.Skills[0].Name, parsed.Skills[0].Name)

	// Hooks round-trip fully
	assert.Equal(t, original.Hooks, parsed.Hooks)
}

func TestCodexConfig_RoundTrip(t *testing.T) {
	original := sampleSettings()

	data, err := original.ToCodexConfig()
	require.NoError(t, err)
	require.NotEmpty(t, data)

	parsed, err := ParseCodexConfig(data)
	require.NoError(t, err)

	// Only agents survive Codex TOML round-trip
	require.Len(t, parsed.Agents, len(original.Agents))
	assert.Equal(t, original.Agents[0].Name, parsed.Agents[0].Name)
	assert.Equal(t, original.Agents[0].Description, parsed.Agents[0].Description)
	assert.Equal(t, original.Agents[0].Model, parsed.Agents[0].Model)
	assert.Equal(t, original.Agents[1].Name, parsed.Agents[1].Name)
}

func TestClaudeJSON_Empty(t *testing.T) {
	ps := &PlatformSettings{}
	data, err := ps.ToClaudeJSON()
	require.NoError(t, err)
	assert.Contains(t, string(data), "{")
}

func TestGeminiJSON_Empty(t *testing.T) {
	ps := &PlatformSettings{}
	data, err := ps.ToGeminiJSON()
	require.NoError(t, err)
	assert.Contains(t, string(data), "{")
}

func TestCodexConfig_Empty(t *testing.T) {
	ps := &PlatformSettings{}
	data, err := ps.ToCodexConfig()
	require.NoError(t, err)
	assert.Empty(t, data)
}

func TestParseTOMLLine(t *testing.T) {
	key, val, ok := parseTOMLLine(`name = "executor"`)
	require.True(t, ok)
	assert.Equal(t, "name", key)
	assert.Equal(t, "executor", val)

	_, _, ok = parseTOMLLine("# comment")
	assert.False(t, ok)
}
