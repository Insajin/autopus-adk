package content_test

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/content"
)

// ompFlatSkillPathRe matches the stage-1-only skill path form. REQ-013 emits
// .agents/skills/<name>/SKILL.md, so this form names a file that never exists.
var ompFlatSkillPathRe = regexp.MustCompile(`\.agents/skills/[a-z0-9-]+\.md`)

// ompDoubledRuleNamespaceRe matches the doubled namespace produced when a source
// already says .claude/rules/autopus/ and the prefix map appends autopus/ again.
var ompDoubledRuleNamespaceRe = regexp.MustCompile(`\.agents/rules/autopus/autopus/`)

func ompContentBodies(t *testing.T, dir string) map[string]string {
	t.Helper()

	entries, err := fs.ReadDir(contentfs.FS, dir)
	require.NoError(t, err)

	bodies := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		raw, readErr := fs.ReadFile(contentfs.FS, dir+"/"+entry.Name())
		require.NoError(t, readErr)
		bodies[dir+"/"+entry.Name()] = string(raw)
	}
	require.NotEmpty(t, bodies)
	return bodies
}

// TestReplacePlatformReferencesOMP_S12_NotANoOp pins the REQ-015 regression this
// SPEC exists for: before omp keys were added, ReplacePlatformReferences
// returned the body untouched.
func TestReplacePlatformReferencesOMP_S12_NotANoOp(t *testing.T) {
	t.Parallel()

	raw, err := fs.ReadFile(contentfs.FS, "rules/doc-storage.md")
	require.NoError(t, err)
	source := string(raw)
	require.Contains(t, source, ".claude/", "fixture precondition: the source names .claude/")

	out := content.ReplacePlatformReferences(source, "omp")

	assert.NotEqual(t, source, out, "omp normalization must not be a no-op")
	assert.Contains(t, out, ".omp/", "the catch-all .claude/ prefix maps to the omp native root")
	assert.NotContains(t, out, ".claude/", "no Claude-native path may survive")
}

// TestReplacePlatformReferencesOMP_S12_PathPrefixes pins each REQ-015 stage-1
// mapping, including the already-namespaced rule path that must not double.
func TestReplacePlatformReferencesOMP_S12_PathPrefixes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"rules bare", "See `.claude/rules/branding.md`.", "`.agents/rules/autopus/branding.md`"},
		{"rules already namespaced", "See `.claude/rules/autopus/branding.md`.", "`.agents/rules/autopus/branding.md`"},
		{"commands", "See `.claude/commands/auto.md`.", "`.agents/commands/auto.md`"},
		{"agents", "See `.claude/agents/autopus/executor.md`.", "`.omp/agents/autopus/executor.md`"},
		{"generic claude dir", "Config lives in `.claude/settings.json`.", "`.omp/settings.json`"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := content.ReplacePlatformReferences(tt.input, "omp")
			assert.Contains(t, got, tt.want)
			assert.NotContains(t, got, ".claude/")
			assert.Empty(t, ompDoubledRuleNamespaceRe.FindAllString(got, -1),
				"the rule namespace must not double")
		})
	}
}

// TestReplacePlatformReferencesOMP_S12_SkillRefFinalForm pins the REQ-015
// stage-2 rewrite that stage-1 prefix mapping alone cannot produce.
func TestReplacePlatformReferencesOMP_S12_SkillRefFinalForm(t *testing.T) {
	t.Parallel()

	got := content.ReplacePlatformReferences("Reference: `.claude/skills/autopus/ax-annotation.md`", "omp")

	assert.Contains(t, got, "`.agents/skills/ax-annotation/SKILL.md`")
	assert.Empty(t, ompFlatSkillPathRe.FindAllString(got, -1),
		"the stage-1-only form .agents/skills/<name>.md must not survive")
}

// TestNormalizeAgentReferencesOMP_S12_BrandingRule pins REQ-015 for the branding
// reference, which fell back to the repository-internal source path while omp
// had no brandingRule entry.
func TestNormalizeAgentReferencesOMP_S12_BrandingRule(t *testing.T) {
	t.Parallel()

	input := "- **브랜딩**: `content/rules/branding.md` 준수"
	got := content.NormalizeAgentReferences(input, "omp")

	assert.Contains(t, got, "`.agents/rules/autopus/branding.md`")
	assert.NotContains(t, got, "`content/rules/branding.md`",
		"omp must not fall back to the repository-internal source path")
}

// TestReplacePlatformReferencesOMP_S12_ToolNames pins TodoWrite -> todo for omp.
func TestReplacePlatformReferencesOMP_S12_ToolNames(t *testing.T) {
	t.Parallel()

	got := content.ReplacePlatformReferences("Track progress with the TodoWrite tool.", "omp")

	assert.Contains(t, got, "Track progress with the todo tool.")
	assert.NotContains(t, got, "TodoWrite")
	assert.NotContains(t, got, "todowrite", "todowrite is the opencode name, not the omp name")
}

// TestReplacePlatformReferencesOMP_S12_ToolNameIsolation guards the other
// platforms against the omp tool-name change. A codex line that already contains
// the word todo must still lose its TodoWrite reference.
func TestReplacePlatformReferencesOMP_S12_ToolNameIsolation(t *testing.T) {
	t.Parallel()

	input := "Use TodoWrite to track todo items."

	codex := content.ReplacePlatformReferences(input, "codex")
	assert.Equal(t, "// TodoWrite is not available on this platform", codex,
		"codex has no TodoWrite equivalent, so the instruction must still be neutralized")

	opencode := content.ReplacePlatformReferences(input, "opencode")
	assert.Contains(t, opencode, "todowrite")
	assert.NotContains(t, opencode, "TodoWrite")
}

// TestReplacePlatformReferencesOMP_S12_WorkflowToolNames pins REQ-015 for the
// Claude-only workflow tools. omp exposes none of them under these names, so
// none may survive into an omp surface.
func TestReplacePlatformReferencesOMP_S12_WorkflowToolNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"AskUserQuestion", "TaskCreate", "TaskUpdate", "TaskList", "TaskGet", "TeamCreate", "SendMessage"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := content.ReplacePlatformReferences("Use the "+name+" tool here.", "omp")
			assert.NotContains(t, got, name)
			assert.NotContains(t, got, "todowrite",
				"todowrite is the opencode name, not the omp name")
		})
	}
}

// TestReplacePlatformReferencesOMP_S12_AgentInvocation pins REQ-015 for the
// Claude agent-invocation syntax.
func TestReplacePlatformReferencesOMP_S12_AgentInvocation(t *testing.T) {
	t.Parallel()

	got := content.ReplacePlatformReferences(`Agent(subagent_type="executor", task="build")`, "omp")

	assert.Contains(t, got, `subagent_type="executor"`)
	assert.Contains(t, got, `prompt="build"`)
	assert.NotContains(t, got, "Agent(subagent_type=")
}

// TestReplacePlatformReferencesOMP_S12_EmittedBodies sweeps the real rule and
// skill sources so no Claude-native path, stage-1-only skill path, or doubled
// rule namespace reaches an omp surface.
func TestReplacePlatformReferencesOMP_S12_EmittedBodies(t *testing.T) {
	t.Parallel()

	bodies := ompContentBodies(t, "rules")
	for name, body := range ompContentBodies(t, "skills") {
		bodies[name] = body
	}

	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out := content.ReplacePlatformReferences(body, "omp")

			assert.NotContains(t, out, ".claude/",
				"%s must not reference a Claude-native path", name)
			assert.Empty(t, ompFlatSkillPathRe.FindAllString(out, -1),
				"%s must reference .agents/skills/<name>/SKILL.md", name)
			assert.Empty(t, ompDoubledRuleNamespaceRe.FindAllString(out, -1),
				"%s must reference .agents/rules/autopus/<name>.md", name)
			assert.NotContains(t, out, "TodoWrite",
				"%s must use the omp tool name todo", name)
		})
	}
}
