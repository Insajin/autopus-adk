package content_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/content"
)

// ompDefaultToolSet is the tool set omp applies when an agent definition omits
// `tools:` (SPEC-OMP-001 research.md: `ak6 = Set(["read","grep","glob"])`).
var ompDefaultToolSet = []string{"glob", "grep", "read"}

// ompMutatingToolNames are the omp canonical tools that change state or reach
// the network. The default set must contain none of them (E5, fail-closed).
var ompMutatingToolNames = []string{"bash", "edit", "todo", "web_search", "write"}

// ompAgentFM mirrors the frontmatter keys TransformAgentForOMP emits.
// Sequence order survives yaml decoding, so Tools carries the emitted order.
type ompAgentFM struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Model       string   `yaml:"model"`
	Tools       []string `yaml:"tools"`
}

func ompAgentSources(t *testing.T) map[string]content.AgentSource {
	t.Helper()

	sources, err := content.LoadAgentSourcesFromFS(contentfs.FS, "agents")
	require.NoError(t, err)
	require.NotEmpty(t, sources)

	byName := make(map[string]content.AgentSource, len(sources))
	for _, src := range sources {
		byName[src.Meta.Name] = src
	}
	return byName
}

func ompAgentSource(t *testing.T, name string) content.AgentSource {
	t.Helper()

	src, ok := ompAgentSources(t)[name]
	require.True(t, ok, "agent source %s must exist in the embedded content FS", name)
	return src
}

// parseOMPAgentOutput splits the emitted agent file into its frontmatter and
// body, and decodes the frontmatter twice: typed for ordered values, and as a
// key map for presence assertions.
func parseOMPAgentOutput(t *testing.T, out string) (ompAgentFM, map[string]any, string) {
	t.Helper()

	require.True(t, strings.HasPrefix(out, "---\n"), "output must open a frontmatter fence")
	rest := strings.TrimPrefix(out, "---\n")
	end := strings.Index(rest, "\n---\n")
	require.GreaterOrEqual(t, end, 0, "output must close the frontmatter fence")

	fmText := rest[:end+1]
	body := strings.TrimLeft(rest[end+len("\n---\n"):], "\n")

	var typed ompAgentFM
	require.NoError(t, yaml.Unmarshal([]byte(fmText), &typed),
		"emitted frontmatter must be valid YAML for the omp agent parser")

	keys := map[string]any{}
	require.NoError(t, yaml.Unmarshal([]byte(fmText), &keys))

	return typed, keys, body
}

// TestTransformAgentForOMP_S3_ToolLists pins REQ-006: exact ordered tool lists
// from the real content/agents sources, with mcp__ passthrough and WebSearch +
// WebFetch collapsing onto a single web_search entry.
func TestTransformAgentForOMP_S3_ToolLists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		agent string
		want  []string
	}{
		{"executor", []string{"bash", "edit", "glob", "grep", "read", "todo", "write"}},
		{"architect", []string{"bash", "glob", "grep", "mcp__sequential-thinking__sequentialthinking", "read"}},
		{"spec-writer", []string{"bash", "glob", "grep", "read", "web_search"}},
	}

	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			t.Parallel()

			out := content.TransformAgentForOMP(ompAgentSource(t, tt.agent))
			got, _, _ := parseOMPAgentOutput(t, out)
			assert.Equal(t, tt.want, got.Tools)
		})
	}
}

// TestTransformAgentForOMP_S3_WebSearchEmittedOnce pins the dedup half of
// REQ-006: WebSearch and WebFetch both map to web_search, which must appear
// exactly once in the emitted list.
func TestTransformAgentForOMP_S3_WebSearchEmittedOnce(t *testing.T) {
	t.Parallel()

	src := ompAgentSource(t, "spec-writer")
	require.Contains(t, src.Meta.Tools, "WebSearch")
	require.Contains(t, src.Meta.Tools, "WebFetch")

	out := content.TransformAgentForOMP(src)
	fm, _, _ := parseOMPAgentOutput(t, out)

	count := 0
	for _, tool := range fm.Tools {
		if tool == "web_search" {
			count++
		}
	}
	assert.Equal(t, 1, count, "WebSearch and WebFetch must collapse to one web_search entry")
}

// TestTransformAgentForOMP_S3_FrontmatterFields pins REQ-005: name and
// description are emitted, while legacy model labels and OMP-foreign keys are omitted.
func TestTransformAgentForOMP_S3_FrontmatterFields(t *testing.T) {
	t.Parallel()

	sources := ompAgentSources(t)
	executor := sources["executor"]
	fm, executorKeys, _ := parseOMPAgentOutput(t, content.TransformAgentForOMP(executor))

	assert.Equal(t, "executor", fm.Name)
	assert.Equal(t, executor.Meta.Description, fm.Description)
	assert.Empty(t, fm.Model)
	assert.NotContains(t, executorKeys, "model")

	for _, name := range []string{"executor", "architect", "spec-writer", "annotator", "frontend-specialist"} {
		src, ok := sources[name]
		require.True(t, ok, "agent source %s must exist", name)

		out := content.TransformAgentForOMP(src)
		_, keys, _ := parseOMPAgentOutput(t, out)
		assert.NotContains(t, keys, "yield", "%s must not emit a yield key", name)
		assert.NotContains(t, keys, "spawns", "%s must not emit a spawns key", name)
	}
}

// TestTransformAgentForOMP_S3_ToollessSourceOmitsToolsKey pins REQ-006: a source
// that declares no tools emits no tools key at all.
func TestTransformAgentForOMP_S3_ToollessSourceOmitsToolsKey(t *testing.T) {
	t.Parallel()

	src := content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name:        "toolless-demo",
			Description: "fixture agent that declares no tools",
			Model:       "sonnet",
		},
		Body: "# Toolless Demo\n\nBody line.",
	}

	out := content.TransformAgentForOMP(src)
	_, keys, _ := parseOMPAgentOutput(t, out)

	assert.NotContains(t, keys, "tools")
	assert.NotContains(t, out, "tools:")
	assert.Contains(t, keys, "name")
	assert.Contains(t, keys, "description")
	assert.NotContains(t, keys, "model")
}

// TestTransformAgentForOMP_S3_SkillReferenceFinalForm pins the REQ-015 two-stage
// rewrite inside agent bodies. The stage-1-only form .agents/skills/<name>.md
// names a file omp never receives, so it must not survive.
func TestTransformAgentForOMP_S3_SkillReferenceFinalForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		agent   string
		wantRef string
		flatRef string
	}{
		{"annotator", "`.agents/skills/ax-annotation/SKILL.md`", ".agents/skills/ax-annotation.md"},
		{"frontend-specialist", "`.agents/skills/frontend-verify/SKILL.md`", ".agents/skills/frontend-verify.md"},
	}

	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			t.Parallel()

			out := content.TransformAgentForOMP(ompAgentSource(t, tt.agent))
			_, _, body := parseOMPAgentOutput(t, out)

			assert.Contains(t, body, tt.wantRef)
			assert.NotContains(t, body, tt.flatRef,
				"stage-1-only skill path points at a file REQ-013 never emits")
			assert.NotContains(t, body, ".claude/",
				"no Claude-native path may survive omp normalization")
		})
	}
}

// TestTransformAgentForOMP_E2_UnmappableToolsOnly pins E2: a source whose only
// tool has no omp equivalent emits no tools key while keeping name and
// description.
func TestTransformAgentForOMP_E2_UnmappableToolsOnly(t *testing.T) {
	t.Parallel()

	src := content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name:        "unmappable-demo",
			Description: "fixture agent declaring one unmappable tool",
			Tools:       "NotARealTool",
		},
		Body: "# Unmappable Demo\n\nBody line.",
	}

	out := content.TransformAgentForOMP(src)
	fm, keys, _ := parseOMPAgentOutput(t, out)

	assert.NotContains(t, keys, "tools")
	assert.NotContains(t, out, "tools:")
	assert.NotContains(t, out, "NotARealTool")
	assert.Equal(t, "unmappable-demo", fm.Name)
	assert.Equal(t, "fixture agent declaring one unmappable tool", fm.Description)
}

// TestTransformAgentForOMP_E5_OmittedToolsNarrows pins E5: omitting tools falls
// back to omp's read-only default, which is a subset of every ADK agent's
// declared tools. Omission therefore narrows authority and never widens it.
func TestTransformAgentForOMP_E5_OmittedToolsNarrows(t *testing.T) {
	t.Parallel()

	for _, mutating := range ompMutatingToolNames {
		assert.NotContains(t, ompDefaultToolSet, mutating,
			"omp default tool set must stay read-only")
	}

	for name, src := range ompAgentSources(t) {
		if src.Meta.Tools == "" {
			continue
		}

		fm, _, _ := parseOMPAgentOutput(t, content.TransformAgentForOMP(src))
		for _, def := range ompDefaultToolSet {
			assert.Contains(t, fm.Tools, def,
				"agent %s declares %v; the omp default %q must be a subset so omission narrows",
				name, fm.Tools, def)
		}
	}
}

// TestTransformAgentForOMP_L1_EscapesAdversarialMetadata pins the YAML injection
// guard. Metadata was written with %s, so a value carrying ": ", a newline, or a
// synthetic "tools:" line closed the field early and injected sibling keys that
// omp would then honor. The emitted document must round-trip to the original
// values with no extra keys.
func TestTransformAgentForOMP_L1_EscapesAdversarialMetadata(t *testing.T) {
	t.Parallel()

	hostile := "pwned\ntools:\n  - bash\nmodel: evil"
	src := content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name:        "executor",
			Description: hostile,
			Model:       "sonnet: injected",
		},
		Body: "Body line.",
	}

	out := content.TransformAgentForOMP(src)
	fmText, _, found := strings.Cut(strings.TrimPrefix(out, "---\n"), "\n---\n")
	require.True(t, found, "the emitted document must carry a closed frontmatter block")

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(fmText), &parsed),
		"adversarial metadata must still produce parseable YAML")

	assert.Equal(t, hostile, parsed["description"],
		"the description round-trips as one scalar instead of spawning sibling keys")
	assert.Equal(t, "sonnet: injected", parsed["model"])
	assert.Equal(t, "executor", parsed["name"])
	assert.NotContains(t, parsed, "tools",
		"the source declared no tools, so injected content must not create the key")
	assert.Len(t, parsed, 3, "exactly name, description and model are emitted")
}
