package pipeline_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// TestPhasePromptBuilder_BuildPrompt_Plan verifies that the Plan phase prompt
// includes spec.md and plan.md file contents.
func TestPhasePromptBuilder_BuildPrompt_Plan(t *testing.T) {
	t.Parallel()

	// Given: a temp directory with spec.md and plan.md
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# SPEC content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plan.md"), []byte("# Plan content"), 0o644))

	builder := pipeline.NewPhasePromptBuilder(dir)

	// When: BuildPrompt is called for the Plan phase
	prompt, err := builder.BuildPrompt(pipeline.PhasePlan, pipeline.PhaseContext{})

	// Then: the prompt contains spec.md content
	require.NoError(t, err)
	assert.Contains(t, prompt, "SPEC content")
}

// TestPhasePromptBuilder_BuildPrompt_Implement_IncludesPlanResult verifies
// that the Implement phase prompt contains the Plan phase result (REQ-2).
func TestPhasePromptBuilder_BuildPrompt_Implement_IncludesPlanResult(t *testing.T) {
	t.Parallel()

	// Given: a temp dir with spec.md
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# SPEC"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acceptance.md"), []byte("### Scenario: Login\nGiven a user\nWhen login\nThen access is granted"), 0o644))

	builder := pipeline.NewPhasePromptBuilder(dir)
	ctx := pipeline.PhaseContext{
		PreviousResults: map[pipeline.PhaseID]string{
			pipeline.PhasePlan: "plan phase output here",
		},
	}

	// When: BuildPrompt is called for the Implement phase
	prompt, err := builder.BuildPrompt(pipeline.PhaseImplement, ctx)

	// Then: the prompt contains the Plan phase output
	require.NoError(t, err)
	assert.Contains(t, prompt, "plan phase output here")
	assert.Contains(t, prompt, "## Acceptance")
	assert.Contains(t, prompt, "Scenario: Login")
}

func TestPhasePromptBuilder_BuildPrompt_TestScaffold_IncludesAcceptance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# SPEC"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acceptance.md"), []byte("### Scenario 1: Login\nGiven a user\nWhen login\nThen access is granted"), 0o644))

	builder := pipeline.NewPhasePromptBuilder(dir)
	prompt, err := builder.BuildPrompt(pipeline.PhaseTestScaffold, pipeline.PhaseContext{})

	require.NoError(t, err)
	assert.Contains(t, prompt, "## Acceptance")
	assert.Contains(t, prompt, "Scenario 1: Login")
}

// TestPhasePromptBuilder_BuildPrompt_Validate_IncludesImplResult verifies
// that the Validate phase prompt contains the Implement phase result (REQ-2).
func TestPhasePromptBuilder_BuildPrompt_Validate_IncludesImplResult(t *testing.T) {
	t.Parallel()

	// Given: a temp dir with spec.md
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# SPEC"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acceptance.md"), []byte("### Edge Case 1: Failure\nGiven a failure\nWhen retrying\nThen the system recovers"), 0o644))

	builder := pipeline.NewPhasePromptBuilder(dir)
	ctx := pipeline.PhaseContext{
		PreviousResults: map[pipeline.PhaseID]string{
			pipeline.PhaseImplement: "implementation output here",
		},
	}

	// When: BuildPrompt is called for the Validate phase
	prompt, err := builder.BuildPrompt(pipeline.PhaseValidate, ctx)

	// Then: the prompt contains the Implement phase output
	require.NoError(t, err)
	assert.Contains(t, prompt, "implementation output here")
	assert.Contains(t, prompt, "## Acceptance")
	assert.Contains(t, prompt, "Edge Case 1: Failure")
}
