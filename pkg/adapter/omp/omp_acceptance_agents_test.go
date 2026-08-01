package omp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	contentfs "github.com/insajin/autopus-adk/content"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

func loadAgentSource(t *testing.T, name string) pkgcontent.AgentSource {
	t.Helper()
	sources, err := pkgcontent.LoadAgentSourcesFromFS(contentfs.FS, "agents")
	require.NoError(t, err)
	for _, src := range sources {
		if src.Meta.Name == name {
			return src
		}
	}
	t.Fatalf("agent source %q not found in content/agents", name)
	return pkgcontent.AgentSource{}
}

// emittedAgentTools parses the emitted frontmatter and returns the tools list.
// The second return value reports whether the `tools` key was emitted at all.
func emittedAgentTools(t *testing.T, emitted string) ([]string, bool) {
	t.Helper()
	fm, _ := splitEmittedFrontmatter(t, emitted)
	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(fm), &parsed))
	raw, ok := parsed["tools"]
	if !ok {
		return nil, false
	}
	items, ok := raw.([]any)
	require.True(t, ok, "tools must be emitted as a sequence, got %T", raw)
	tools := make([]string, 0, len(items))
	for _, item := range items {
		tools = append(tools, item.(string))
	}
	return tools, true
}

// TestOMPAcceptance_S3_AgentToolMapping covers REQ-006 canonical tool translation.
func TestOMPAcceptance_S3_AgentToolMapping(t *testing.T) {
	tests := []struct {
		name  string
		agent string
		want  []string
	}{
		{"executor maps TodoWrite to todo", "executor",
			[]string{"bash", "edit", "glob", "grep", "read", "todo", "write"}},
		{"architect passes mcp__ through unchanged", "architect",
			[]string{"bash", "glob", "grep", "mcp__sequential-thinking__sequentialthinking", "read"}},
		{"spec-writer dedups WebSearch and WebFetch", "spec-writer",
			[]string{"bash", "glob", "grep", "read", "web_search"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitted := pkgcontent.TransformAgentForOMP(loadAgentSource(t, tt.agent))
			got, ok := emittedAgentTools(t, emitted)
			require.True(t, ok, "%s must emit a tools key", tt.agent)
			assert.Equal(t, tt.want, got,
				"%s tools must be deduplicated and alphabetically sorted", tt.agent)
		})
	}

	// web_search is emitted once even though WebSearch and WebFetch both map to it.
	specWriter := pkgcontent.TransformAgentForOMP(loadAgentSource(t, "spec-writer"))
	specFM, _ := splitEmittedFrontmatter(t, specWriter)
	assert.Equal(t, 1, strings.Count(specFM, "web_search"))
}

// TestOMPAcceptance_S3_AgentFrontmatterContract covers REQ-005 metadata emission.
func TestOMPAcceptance_S3_AgentFrontmatterContract(t *testing.T) {
	src := loadAgentSource(t, "executor")
	emitted := pkgcontent.TransformAgentForOMP(src)
	fm, _ := splitEmittedFrontmatter(t, emitted)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(fm), &parsed))
	assert.Equal(t, "executor", parsed["name"])
	assert.Equal(t, src.Meta.Description, parsed["description"])
	assert.Equal(t, "sonnet", parsed["model"])

	// yield and spawns are filled in by the omp parser and must not be emitted.
	for _, agent := range []string{"executor", "architect", "spec-writer", "annotator", "frontend-specialist"} {
		agentFM, _ := splitEmittedFrontmatter(t, pkgcontent.TransformAgentForOMP(loadAgentSource(t, agent)))
		var agentParsed map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(agentFM), &agentParsed))
		assert.NotContains(t, agentParsed, "yield", "%s must not emit yield", agent)
		assert.NotContains(t, agentParsed, "spawns", "%s must not emit spawns", agent)
	}
}

// TestOMPAcceptance_S3_AgentBodyNormalization covers REQ-005/REQ-015 body rewriting.
func TestOMPAcceptance_S3_AgentBodyNormalization(t *testing.T) {
	tests := []struct {
		agent     string
		wantRef   string
		staleRef  string
		skillName string
	}{
		{"annotator", "`.agents/skills/ax-annotation/SKILL.md`", "`.agents/skills/ax-annotation.md`", "ax-annotation"},
		{"frontend-specialist", "`.agents/skills/frontend-verify/SKILL.md`", "`.agents/skills/frontend-verify.md`", "frontend-verify"},
	}

	dir := generateOMPOnly(t)

	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			emitted := pkgcontent.TransformAgentForOMP(loadAgentSource(t, tt.agent))
			_, body := splitEmittedFrontmatter(t, emitted)

			assert.Contains(t, body, tt.wantRef)
			assert.NotContains(t, body, tt.staleRef,
				"stage-2 skill path rewrite must replace the flat .md form")
			assert.NotContains(t, body, ".claude/",
				"no Claude-native path may survive omp normalization")

			// The referenced path must match the file the skill surface actually emits.
			emittedSkill := filepath.Join(dir, ".agents", "skills", tt.skillName, "SKILL.md")
			assert.FileExists(t, emittedSkill,
				"agent body points at %s so REQ-013 must emit that exact path", tt.wantRef)
		})
	}
}

// TestOMPAcceptance_E2_UnmappedToolsOnly covers REQ-006 fail-closed omission.
func TestOMPAcceptance_E2_UnmappedToolsOnly(t *testing.T) {
	src := pkgcontent.AgentSource{
		Meta: pkgcontent.AgentSourceMeta{
			Name:        "toolless-demo",
			Description: "demo agent",
			Tools:       "NotARealTool",
		},
		Body: "Demo body.",
	}
	emitted := pkgcontent.TransformAgentForOMP(src)

	_, ok := emittedAgentTools(t, emitted)
	assert.False(t, ok, "an agent with no mappable tool must omit the tools key")

	fm, _ := splitEmittedFrontmatter(t, emitted)
	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(fm), &parsed))
	assert.Equal(t, "toolless-demo", parsed["name"])
	assert.Equal(t, "demo agent", parsed["description"])
}

// TestOMPAcceptance_S3_ToollessAgentOmitsKey covers REQ-006 when no tools are declared.
func TestOMPAcceptance_S3_ToollessAgentOmitsKey(t *testing.T) {
	src := pkgcontent.AgentSource{
		Meta: pkgcontent.AgentSourceMeta{Name: "toolless-demo", Description: "demo"},
		Body: "Demo body.",
	}
	_, ok := emittedAgentTools(t, pkgcontent.TransformAgentForOMP(src))
	assert.False(t, ok)
}

// TestOMPAcceptance_S6_AgentSurfaceCount covers REQ-005 emission scope.
func TestOMPAcceptance_S6_AgentSurfaceCount(t *testing.T) {
	dir := generateOMPOnly(t)

	sources, err := pkgcontent.LoadAgentSourcesFromFS(contentfs.FS, "agents")
	require.NoError(t, err)
	want := make(map[string]bool, len(sources))
	for _, src := range sources {
		want[src.Meta.Name+".md"] = true
	}
	assert.Len(t, want, 16, "content/agents must hold exactly 16 agent sources")

	entries, err := os.ReadDir(filepath.Join(dir, ".omp", "agents"))
	require.NoError(t, err)
	got := make(map[string]bool, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			got[e.Name()] = true
		}
	}
	assert.Equal(t, want, got, ".omp/agents/ file name set must equal content/agents/")
}
