package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestGenerateAllTemplates(t *testing.T) {
	t.Parallel()

	contentDir := t.TempDir()
	templateDir := t.TempDir()

	// Create sample agent source
	agentDir := filepath.Join(contentDir, "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0755))
	agentMD := `---
name: test-agent
description: A test agent
model: sonnet
skills:
  - tdd
---

# Test Agent

## Role

Does testing.

## Workflow

1. Run tests
`
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "test-agent.md"), []byte(agentMD), 0644))

	// Create sample skill source
	skillDir := filepath.Join(contentDir, "skills")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	skillMD := `---
name: test-skill
description: A test skill
triggers:
  - test
category: testing
---

# Test Skill

Use .claude/rules/ for guidelines.
`
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "test-skill.md"), []byte(skillMD), 0644))

	// Run generator
	err := content.GenerateAllTemplates(contentDir, templateDir)
	require.NoError(t, err)

	// Verify Codex agent TOML exists and is rich
	codexAgent := filepath.Join(templateDir, "codex", "agents", "test-agent.toml.tmpl")
	data, err := os.ReadFile(codexAgent)
	require.NoError(t, err)
	assert.Greater(t, len(data), 200, "Codex agent TOML should be >= 200 chars")
	assert.Contains(t, string(data), `name = "test-agent"`)
	assert.Contains(t, string(data), "[developer_instructions]")

	// Verify Gemini agent MD exists
	geminiAgent := filepath.Join(templateDir, "gemini", "agents", "test-agent.md.tmpl")
	data, err = os.ReadFile(geminiAgent)
	require.NoError(t, err)
	assert.Contains(t, string(data), "name: auto-agent-test-agent")
	assert.Contains(t, string(data), "## Role")

	// Verify Codex skill template
	codexSkill := filepath.Join(templateDir, "codex", "skills", "test-skill.md.tmpl")
	data, err = os.ReadFile(codexSkill)
	require.NoError(t, err)
	assert.Contains(t, string(data), "# auto-test-skill")
	assert.Contains(t, string(data), ".codex/rules/")
	assert.NotContains(t, string(data), ".claude/")

	// Verify Gemini skill template (subdirectory structure)
	geminiSkill := filepath.Join(templateDir, "gemini", "skills", "test-skill", "SKILL.md.tmpl")
	data, err = os.ReadFile(geminiSkill)
	require.NoError(t, err)
	assert.Contains(t, string(data), "name: auto-test-skill")
	assert.Contains(t, string(data), ".gemini/rules/")
	assert.NotContains(t, string(data), ".claude/")
}

func TestGenerateAllTemplates_PreservesAutoSkills(t *testing.T) {
	t.Parallel()

	contentDir := t.TempDir()
	templateDir := t.TempDir()

	// Create empty agent dir (no agents)
	require.NoError(t, os.MkdirAll(filepath.Join(contentDir, "agents"), 0755))

	// Create a skill named "auto-fix" to test skip logic
	skillDir := filepath.Join(contentDir, "skills")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	autoSkillMD := `---
name: auto-fix
description: Should be skipped
---

# Auto Fix
`
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "auto-fix.md"), []byte(autoSkillMD), 0644))

	// Pre-create an existing auto-fix template
	existingDir := filepath.Join(templateDir, "codex", "skills")
	require.NoError(t, os.MkdirAll(existingDir, 0755))
	existingContent := "# existing auto-fix — do not overwrite\n"
	require.NoError(t, os.WriteFile(filepath.Join(existingDir, "auto-fix.md.tmpl"), []byte(existingContent), 0644))

	err := content.GenerateAllTemplates(contentDir, templateDir)
	require.NoError(t, err)

	// Existing auto-fix template should be preserved
	data, err := os.ReadFile(filepath.Join(existingDir, "auto-fix.md.tmpl"))
	require.NoError(t, err)
	assert.Equal(t, existingContent, string(data), "auto-fix template should not be overwritten")
}

func TestGenerateAllTemplates_EmptyContent(t *testing.T) {
	t.Parallel()

	contentDir := t.TempDir()
	templateDir := t.TempDir()

	// Create empty subdirs
	require.NoError(t, os.MkdirAll(filepath.Join(contentDir, "agents"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(contentDir, "skills"), 0755))

	err := content.GenerateAllTemplates(contentDir, templateDir)
	require.NoError(t, err)

	// Directories should be created even with no content
	_, err = os.Stat(filepath.Join(templateDir, "codex", "agents"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(templateDir, "gemini", "agents"))
	assert.NoError(t, err)
}
