package content_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

// TestLoadAgentSourcesFromFS_ReadFileError verifies error on unreadable file.
func TestLoadAgentSourcesFromFS_ReadFileError(t *testing.T) {
	t.Parallel()

	// fstest.MapFS with nil Data triggers a read error for some setups,
	// but the simpler approach is to test with invalid YAML in frontmatter.
	fsys := fstest.MapFS{
		"agents/bad.md": {Data: []byte("---\nname: [invalid\n---\n\nbody")},
	}

	_, err := content.LoadAgentSourcesFromFS(fsys, "agents")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad.md")
}

// TestLoadAgentSourcesFromFS_InvalidDir verifies error on nonexistent directory.
func TestLoadAgentSourcesFromFS_InvalidDir(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{}
	_, err := content.LoadAgentSourcesFromFS(fsys, "nonexistent")
	assert.Error(t, err)
}

// TestLoadAgentSourcesFromFS_EmptyDir returns nil sources for empty directory.
func TestLoadAgentSourcesFromFS_EmptyDir(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"agents/readme.txt": {Data: []byte("not markdown")},
	}
	sources, err := content.LoadAgentSourcesFromFS(fsys, "agents")
	require.NoError(t, err)
	assert.Empty(t, sources)
}

// TestTransformAgentForCodex_PreservesStructuredBody verifies rich markdown survives conversion.
func TestTransformAgentForCodex_PreservesStructuredBody(t *testing.T) {
	t.Parallel()

	src := content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name:        "test",
			Description: "Test agent",
			Model:       "sonnet",
		},
		Body: "## Section\n\nKeep this line.\n\n```go\nfunc main() {}\n```\n\nAlso keep this.",
	}

	result := content.TransformAgentForCodex(src)
	assert.Contains(t, result, "Keep this line.")
	assert.Contains(t, result, "Also keep this.")
	assert.Contains(t, result, "func main()")
	assert.Contains(t, result, `developer_instructions = """`)
}

// TestTransformAgentForCodex_PreservesHeaders verifies section headers remain intact.
func TestTransformAgentForCodex_PreservesHeaders(t *testing.T) {
	t.Parallel()

	src := content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name:        "h1test",
			Description: "H1 test agent",
			Model:       "sonnet",
		},
		Body: "# Title Header\n\n## Subtitle\n\nContent line.",
	}

	result := content.TransformAgentForCodex(src)
	assert.Contains(t, result, "Title Header")
	assert.Contains(t, result, "## Subtitle")
	assert.Contains(t, result, "Content line.")
}

// TestTransformAgentForCodex_CodeOnlyBody verifies code-heavy bodies still render valid TOML.
func TestTransformAgentForCodex_CodeOnlyBody(t *testing.T) {
	t.Parallel()

	src := content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name:        "codeonly",
			Description: "Only code",
			Model:       "haiku",
		},
		Body: "```\nsome code\nmore code\n```",
	}

	result := content.TransformAgentForCodex(src)
	// Should still produce valid TOML with developer_instructions
	assert.Contains(t, result, `name = "codeonly"`)
	assert.Contains(t, result, "developer_instructions =")
	assert.Contains(t, result, "some code")
}

// TestTransformAgentForCodex_NoSkills verifies no skills reference in output.
func TestTransformAgentForCodex_NoSkills(t *testing.T) {
	t.Parallel()

	src := content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name:        "noskill",
			Description: "No skills",
			Model:       "sonnet",
		},
		Body: "## Work\n\nDo stuff.",
	}

	result := content.TransformAgentForCodex(src)
	assert.NotContains(t, result, "Skills:")
	assert.Contains(t, result, "Do stuff.")
}

// TestTransformAgentForGemini_NoSkills verifies no skills section in Gemini output.
func TestTransformAgentForGemini_NoSkills(t *testing.T) {
	t.Parallel()

	src := content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name:        "noskill",
			Description: "No skills",
			Model:       "sonnet",
		},
		Body: "## Work\n\nDo stuff.",
	}

	result := content.TransformAgentForGemini(src)
	assert.NotContains(t, result, "skills:")
	assert.Contains(t, result, "Do stuff.")
}

// TestNewSkillTransformerFromFS verifies FS-based skill loading (0% coverage).
func TestNewSkillTransformerFromFS(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"skills/tdd.md": {Data: []byte("---\nname: tdd\ndescription: TDD\ntriggers:\n  - test\ncategory: quality\n---\n\n# TDD\n\nContent.")},
		"skills/ddd.md": {Data: []byte("---\nname: ddd\ndescription: DDD\ncategory: design\n---\n\n# DDD\n\nDesign content.")},
	}

	transformer, err := content.NewSkillTransformerFromFS(fsys, "skills")
	require.NoError(t, err)

	skills, report, err := transformer.TransformForPlatform("codex")
	require.NoError(t, err)
	assert.Len(t, skills, 2)
	assert.Len(t, report.Compatible, 2)

	// Verify content transformation happened
	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}
	assert.True(t, names["tdd"])
	assert.True(t, names["ddd"])
}

// TestNewSkillTransformerFromFS_InvalidDir returns empty transformer.
func TestNewSkillTransformerFromFS_InvalidDir(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{}
	transformer, err := content.NewSkillTransformerFromFS(fsys, "nonexistent")
	require.NoError(t, err) // returns empty transformer, no error

	skills, _, err := transformer.TransformForPlatform("codex")
	require.NoError(t, err)
	assert.Empty(t, skills)
}

// TestNewSkillTransformerFromFS_InvalidYAML returns error on bad frontmatter.
func TestNewSkillTransformerFromFS_InvalidYAML(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"skills/bad.md": {Data: []byte("---\nname: [invalid yaml\n---\n\nbody")},
	}

	_, err := content.NewSkillTransformerFromFS(fsys, "skills")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad.md")
}

// TestNewSkillTransformerFromFS_SkipsNonMD verifies only .md files are loaded.
func TestNewSkillTransformerFromFS_SkipsNonMD(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"skills/readme.txt":  {Data: []byte("ignore")},
		"skills/config.yaml": {Data: []byte("ignore")},
		"skills/valid.md":    {Data: []byte("---\nname: valid\n---\n\nbody")},
	}

	transformer, err := content.NewSkillTransformerFromFS(fsys, "skills")
	require.NoError(t, err)

	skills, _, err := transformer.TransformForPlatform("codex")
	require.NoError(t, err)
	assert.Len(t, skills, 1)
	assert.Equal(t, "valid", skills[0].Name)
}
