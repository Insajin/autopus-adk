package content_test

import (
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformAgentForGemini_ProjectsKnownNativeToolsAndOmitsUnsupported(t *testing.T) {
	t.Parallel()

	source := content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name:        "tool-projection",
			Description: "Gemini tool projection fixture",
			Tools:       "Read, Write, Edit, Grep, Glob, Bash, TodoWrite, Agent, MysteryExtension",
			Skills:      []string{"tdd", "verification"},
		},
		Body: "## Contract\n\nCodex guidance remains advisory.\n\nBody stays byte-stable.",
	}

	rendered := content.TransformAgentForGemini(source)
	frontmatter, body := decodeGeminiAgent(t, rendered)

	assert.Equal(t, []string{
		"read_file",
		"write_file",
		"replace",
		"grep_search",
		"glob",
		"run_shell_command",
	}, frontmatter.Tools)
	assert.Equal(t, source.Meta.Skills, frontmatter.Skills)
	assert.Equal(t, strings.TrimSpace(source.Body), strings.TrimSpace(body))
	assert.NotContains(t, rendered, "TodoWrite")
	assert.NotContains(t, rendered, "MysteryExtension")
	assert.NotContains(t, rendered, "\n  - Agent\n")
	assert.NotContains(t, rendered, "Codex native enforcement")
}

func TestContextEvolutionExamples_CoverThreeFailureBoundariesWithoutSensitiveData(t *testing.T) {
	t.Parallel()

	data, err := contentfs.FS.ReadFile("skills/agent-pipeline.md")
	require.NoError(t, err)
	section := markdownSection(string(data), "## Context Evolution Examples")
	require.NotEmpty(t, section, "canonical context evolution examples section is required")

	itemPattern := regexp.MustCompile(`(?m)^\d+\.\s+`)
	assert.Len(t, itemPattern.FindAllStringIndex(section, -1), 3)
	for _, required := range []string{
		"raw-body replay",
		"malformed receipt evidence",
		"unsupported tool enforcement",
	} {
		assert.Contains(t, section, required)
	}
	for _, forbidden := range []string{
		"sk-proj-",
		"AKIA",
		"/Users/",
		"/home/",
		"C:\\",
	} {
		assert.NotContains(t, section, forbidden)
	}
}

type geminiAgentFrontmatter struct {
	Name   string   `yaml:"name"`
	Skills []string `yaml:"skills"`
	Tools  []string `yaml:"tools"`
}

func decodeGeminiAgent(t *testing.T, rendered string) (geminiAgentFrontmatter, string) {
	t.Helper()
	parts := strings.SplitN(rendered, "---", 3)
	require.Len(t, parts, 3)
	var frontmatter geminiAgentFrontmatter
	require.NoError(t, yaml.Unmarshal([]byte(parts[1]), &frontmatter))
	return frontmatter, parts[2]
}

func markdownSection(body, heading string) string {
	start := strings.Index(body, heading)
	if start < 0 {
		return ""
	}
	section := body[start+len(heading):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return strings.TrimSpace(section)
}
