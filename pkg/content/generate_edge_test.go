package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestGenerateAllTemplates_InvalidAgentDir(t *testing.T) {
	t.Parallel()

	// contentDir has no agents/ subdirectory
	contentDir := t.TempDir()
	templateDir := t.TempDir()

	err := content.GenerateAllTemplates(contentDir, templateDir)
	// Should fail because agents/ subdir does not exist
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent templates")
}

func TestGenerateAllTemplates_InvalidSkillDir(t *testing.T) {
	t.Parallel()

	contentDir := t.TempDir()
	templateDir := t.TempDir()

	// Create valid agents/ (empty) but no skills/ dir
	require.NoError(t, os.MkdirAll(filepath.Join(contentDir, "agents"), 0755))

	err := content.GenerateAllTemplates(contentDir, templateDir)
	// Should succeed: NewSkillTransformer returns empty transformer on ReadDir error
	assert.NoError(t, err)
}

func TestGenerateAllTemplates_ReadOnlyTemplateDir(t *testing.T) {
	t.Parallel()

	contentDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(contentDir, "agents"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(contentDir, "skills"), 0755))

	// Create a read-only template dir to trigger MkdirAll failure
	templateDir := t.TempDir()
	readOnlyDir := filepath.Join(templateDir, "codex")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0755))

	// Write an agent so the code actually tries to create codex/agents subdir
	agentMD := "---\nname: test\ndescription: test\nmodel: sonnet\n---\n\nBody"
	require.NoError(t, os.WriteFile(
		filepath.Join(contentDir, "agents", "test.md"),
		[]byte(agentMD), 0644,
	))

	// Make codex dir non-writable to force MkdirAll failure on agents subdir
	require.NoError(t, os.Chmod(readOnlyDir, 0444))
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0755) })

	err := content.GenerateAllTemplates(contentDir, templateDir)
	assert.Error(t, err)
}

func TestGenerateAllTemplates_MultipleAgentsAndSkills(t *testing.T) {
	t.Parallel()

	contentDir := t.TempDir()
	templateDir := t.TempDir()

	agentDir := filepath.Join(contentDir, "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0755))

	// Two agents with different models
	for _, a := range []struct {
		name, model string
	}{
		{"planner", "opus"},
		{"reviewer", "haiku"},
	} {
		md := "---\nname: " + a.name + "\ndescription: " + a.name + " agent\nmodel: " + a.model + "\n---\n\n# " + a.name + "\n\nBody."
		require.NoError(t, os.WriteFile(filepath.Join(agentDir, a.name+".md"), []byte(md), 0644))
	}

	skillDir := filepath.Join(contentDir, "skills")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	skillMD := "---\nname: tdd\ndescription: TDD skill\n---\n\n# TDD\n\nContent."
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "tdd.md"), []byte(skillMD), 0644))

	err := content.GenerateAllTemplates(contentDir, templateDir)
	require.NoError(t, err)

	// Verify both agents produced codex + gemini files
	for _, name := range []string{"planner", "reviewer"} {
		codexPath := filepath.Join(templateDir, "codex", "agents", name+".toml.tmpl")
		data, err := os.ReadFile(codexPath)
		require.NoError(t, err, "codex agent %s should exist", name)
		assert.Contains(t, string(data), `name = "`+name+`"`)

		geminiPath := filepath.Join(templateDir, "gemini", "agents", name+".md.tmpl")
		data, err = os.ReadFile(geminiPath)
		require.NoError(t, err, "gemini agent %s should exist", name)
		assert.Contains(t, string(data), "name: auto-agent-"+name)
	}

	// Verify skill produced both platform templates
	codexSkill := filepath.Join(templateDir, "codex", "skills", "tdd.md.tmpl")
	_, err = os.Stat(codexSkill)
	assert.NoError(t, err, "codex tdd skill should exist")

	geminiSkill := filepath.Join(templateDir, "gemini", "skills", "tdd", "SKILL.md.tmpl")
	_, err = os.Stat(geminiSkill)
	assert.NoError(t, err, "gemini tdd skill should exist")
}
