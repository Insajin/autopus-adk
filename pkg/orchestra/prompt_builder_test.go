package orchestra

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/insajin/autopus-adk/templates"
)

func newTestPromptData() PromptData {
	return PromptData{
		ProjectName:    "test-project",
		ProjectSummary: "A test project for unit tests",
		TechStack:      "Go",
		Components:     []string{"pkg/core", "cmd/cli"},
		MustReadFiles:  []string{"ARCHITECTURE.md", "go.mod"},
		RelevantPaths: []RelevantPath{
			{Path: "pkg/core/main.go", Description: "entry point"},
		},
		TargetModule: "pkg/core",
		MaxTurns:     20,
		Topic:        "improve error handling",
		SchemaMethod: "prompt",
		SchemaJSON:   `{"type":"object"}`,
	}
}

func TestNewPromptBuilder(t *testing.T) {
	t.Parallel()
	pb, err := NewPromptBuilder()
	require.NoError(t, err)
	require.NotNil(t, pb)
}

func TestPromptBuilder_BuildDebaterR1(t *testing.T) {
	t.Parallel()
	pb, err := NewPromptBuilder()
	require.NoError(t, err)

	data := newTestPromptData()
	result, err := pb.BuildDebaterR1(data)
	require.NoError(t, err)

	assert.Contains(t, result, "Independent Analyst")
	assert.Contains(t, result, "test-project")
	assert.Contains(t, result, "improve error handling")
	assert.Contains(t, result, "ARCHITECTURE.md")
	assert.Contains(t, result, "pkg/core/main.go")
	assert.Contains(t, result, `{"type":"object"}`)
	assert.Contains(t, result, "Do NOT stop early")
}

func TestPromptBuilder_BuildDebaterR1_NoSchema(t *testing.T) {
	t.Parallel()
	pb, err := NewPromptBuilder()
	require.NoError(t, err)

	data := newTestPromptData()
	data.SchemaMethod = ""
	result, err := pb.BuildDebaterR1(data)
	require.NoError(t, err)

	assert.NotContains(t, result, "Required JSON structure")
}

func TestPromptBuilder_BuildDebaterR2(t *testing.T) {
	t.Parallel()
	pb, err := NewPromptBuilder()
	require.NoError(t, err)

	data := newTestPromptData()
	data.Round = 2
	data.PreviousRound = 1
	data.PreviousResults = []PreviousResult{
		{Alias: "Analyst A", Output: "idea about caching"},
		{Alias: "Analyst B", Output: "idea about retries"},
	}

	result, err := pb.BuildDebaterR2(data)
	require.NoError(t, err)

	assert.Contains(t, result, "Cross-Pollination")
	assert.Contains(t, result, "Analyst A")
	assert.Contains(t, result, "idea about caching")
	assert.Contains(t, result, "Analyst B")
	assert.Contains(t, result, "Do NOT stop early")
}

func TestPromptBuilder_BuildJudge(t *testing.T) {
	t.Parallel()
	pb, err := NewPromptBuilder()
	require.NoError(t, err)

	data := newTestPromptData()
	data.AllResults = []JudgeResult{
		{Alias: "Debater 1", Round1: "r1 analysis", Round2: "r2 synthesis"},
		{Alias: "Debater 2", Round1: "r1 analysis 2", Round2: ""},
	}

	result, err := pb.BuildJudge(data)
	require.NoError(t, err)

	assert.Contains(t, result, "Final Judge")
	assert.Contains(t, result, "Debater 1")
	assert.Contains(t, result, "r1 analysis")
	assert.Contains(t, result, "r2 synthesis")
	assert.Contains(t, result, "Debater 2")
	assert.Contains(t, result, "Do NOT stop early")
}

func TestPromptBuilder_BuildReviewer(t *testing.T) {
	t.Parallel()
	pb, err := NewPromptBuilder()
	require.NoError(t, err)

	data := newTestPromptData()
	data.SpecContent = "## Requirements\n- P0: Must validate input"
	data.CodeContext = "func Validate(s string) error { return nil }"

	result, err := pb.BuildReviewer(data)
	require.NoError(t, err)

	assert.Contains(t, result, "Independent Reviewer")
	assert.Contains(t, result, "P0: Must validate input")
	assert.Contains(t, result, "Pre-collected Code Context")
	assert.Contains(t, result, "func Validate")
	assert.Contains(t, result, "Do NOT stop early")
}

func TestPromptBuilder_BuildReviewer_NoCodeContext(t *testing.T) {
	t.Parallel()
	pb, err := NewPromptBuilder()
	require.NoError(t, err)

	data := newTestPromptData()
	data.SpecContent = "## Requirements\n- P0: test"
	data.CodeContext = ""

	result, err := pb.BuildReviewer(data)
	require.NoError(t, err)

	assert.NotContains(t, result, "Pre-collected Code Context")
}

func TestPromptBuilder_BuildReviewerWithManifest(t *testing.T) {
	t.Parallel()
	pb, err := NewPromptBuilder()
	require.NoError(t, err)

	data := newTestPromptData()
	data.SpecContent = "## Requirements\n- Must expose manifest"
	data.CodeContext = "func BuildPrompt() {}"
	data.SnapshotID = "snapshot-a"
	data.SnapshotContent = "quality recall entry"
	data.SnapshotSourceRefs = []string{"quality:L-001"}

	result, manifest, err := pb.BuildReviewerWithManifest(data)
	require.NoError(t, err)

	assert.Contains(t, result, "Must expose manifest")
	assert.Equal(t, "snapshot-a", manifest.SnapshotID)
	assert.Contains(t, manifestEntryIDs(manifest), "orchestra:reviewer:identity")
	assert.Contains(t, manifestEntryIDs(manifest), "orchestra:project-context")
	assert.Contains(t, manifestEntryIDs(manifest), "snapshot-a")
	assert.Contains(t, manifestEntryIDs(manifest), "orchestra:reviewer:task")
	assert.False(t, manifestEntryByID(manifest, "orchestra:reviewer:task").CacheEligible)
}

func TestPromptBuilder_StableManifestHashesTemplateContent(t *testing.T) {
	t.Parallel()

	data := newTestPromptData()
	manifest, err := buildPromptManifest("reviewer", "shared/orchestra-reviewer.md.tmpl", data)
	require.NoError(t, err)

	contextBody, err := templates.FS.ReadFile("shared/orchestra-context.md.tmpl")
	require.NoError(t, err)
	roleBody, err := templates.FS.ReadFile("shared/orchestra-reviewer.md.tmpl")
	require.NoError(t, err)
	expectedContent := strings.Join([]string{
		"template: shared/orchestra-context.md.tmpl\n" + string(contextBody) + "\n",
		"template: shared/orchestra-reviewer.md.tmpl\n" + string(roleBody) + "\n",
	}, "")
	expected, err := promptlayer.Render([]promptlayer.Layer{{
		ID:            "orchestra:reviewer:identity",
		Kind:          promptlayer.KindStable,
		Group:         promptlayer.GroupIdentityRules,
		SourceRef:     "shared/orchestra-reviewer.md.tmpl",
		Content:       expectedContent,
		CacheEligible: true,
	}})
	require.NoError(t, err)

	assert.Equal(t, expected.Manifest.Entries[0].Hash, manifestEntryByID(manifest, "orchestra:reviewer:identity").Hash)
	assert.Contains(t, expectedContent, "prompt layer manifest")
}

func TestPromptBuilder_SanitizesUnsafeSnapshotManifestRefs(t *testing.T) {
	t.Parallel()

	manifest, err := buildPromptManifest("reviewer", "shared/orchestra-reviewer.md.tmpl", PromptData{
		SnapshotID:         "/Users/example/.ssh/id_rsa",
		SnapshotContent:    "digest-only content",
		SnapshotSourceRefs: []string{"/Users/example/.ssh/id_rsa", "quality:L-001", "token=sk-proj-abcdefghijklmnopqrstuvwxyz"},
	})
	require.NoError(t, err)

	assert.NotContains(t, manifest.SnapshotID, "/Users")
	entry := manifestEntryByID(manifest, manifest.SnapshotID)
	assert.NotContains(t, entry.SourceRef, "/Users")
	assert.NotContains(t, entry.SourceRef, "sk-proj")
	assert.Contains(t, entry.SourceRef, "quality:L-001")
	assert.Contains(t, entry.SourceRef, "snapshot-ref:")
}

func TestPromptBuilder_ContextInjected(t *testing.T) {
	t.Parallel()
	pb, err := NewPromptBuilder()
	require.NoError(t, err)

	data := newTestPromptData()
	result, err := pb.BuildDebaterR1(data)
	require.NoError(t, err)

	// Context template sections should be present.
	assert.Contains(t, result, "Project Context")
	assert.Contains(t, result, "Step 1: Understand the Project")
	assert.Contains(t, result, "Step 2: Explore Relevant Code")

	// Components should be rendered.
	assert.Contains(t, result, "pkg/core")
	assert.Contains(t, result, "cmd/cli")
}

func TestPromptBuilder_MultipleComponents(t *testing.T) {
	t.Parallel()
	pb, err := NewPromptBuilder()
	require.NoError(t, err)

	data := newTestPromptData()
	data.Components = []string{"alpha", "beta", "gamma"}
	result, err := pb.BuildDebaterR1(data)
	require.NoError(t, err)

	for _, c := range data.Components {
		assert.True(t, strings.Contains(result, c), "missing component: %s", c)
	}
}

func manifestEntryIDs(manifest PromptManifest) []string {
	ids := make([]string, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		ids = append(ids, entry.ID)
	}
	return ids
}

func manifestEntryByID(manifest PromptManifest, id string) PromptManifestEntry {
	for _, entry := range manifest.Entries {
		if entry.ID == id {
			return entry
		}
	}
	return PromptManifestEntry{}
}
