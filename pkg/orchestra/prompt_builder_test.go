package orchestra

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
