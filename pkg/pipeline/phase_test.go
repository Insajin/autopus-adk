package pipeline_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// TestDefaultPhases_Returns5Phases verifies that DefaultPhases returns exactly
// 5 pipeline phase definitions.
func TestDefaultPhases_Returns5Phases(t *testing.T) {
	t.Parallel()

	// When: DefaultPhases is called
	phases := pipeline.DefaultPhases()

	// Then: exactly 5 phases are returned
	assert.Len(t, phases, 5)
}

// TestDefaultPhases_CorrectOrder verifies that DefaultPhases returns phases in
// the canonical order: Plan → TestScaffold → Implement → Validate → Review.
func TestDefaultPhases_CorrectOrder(t *testing.T) {
	t.Parallel()

	// When: DefaultPhases is called
	phases := pipeline.DefaultPhases()

	// Then: phases are in the expected order
	require.Len(t, phases, 5)
	assert.Equal(t, pipeline.PhasePlan, phases[0].ID)
	assert.Equal(t, pipeline.PhaseTestScaffold, phases[1].ID)
	assert.Equal(t, pipeline.PhaseImplement, phases[2].ID)
	assert.Equal(t, pipeline.PhaseValidate, phases[3].ID)
	assert.Equal(t, pipeline.PhaseReview, phases[4].ID)
}

// TestNormalizeOutput_Claude verifies that NormalizeOutput extracts the
// last_assistant_message field from claude-formatted JSON output.
func TestNormalizeOutput_Claude(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "extracts last_assistant_message",
			input:    `{"last_assistant_message": "plan complete"}`,
			expected: "plan complete",
		},
		{
			name:     "nested last_assistant_message",
			input:    `{"other": "field", "last_assistant_message": "result text"}`,
			expected: "result text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// When: NormalizeOutput is called with claude format
			got := pipeline.NormalizeOutput("claude", tt.input)

			// Then: last_assistant_message is extracted
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestNormalizeOutput_Codex verifies that NormalizeOutput extracts the text
// field from codex-formatted JSON output.
func TestNormalizeOutput_Codex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "extracts text field",
			input:    `{"text": "codex result"}`,
			expected: "codex result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// When: NormalizeOutput is called with codex format
			got := pipeline.NormalizeOutput("codex", tt.input)

			// Then: text field is extracted
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestNormalizeOutput_Gemini verifies that NormalizeOutput extracts the
// prompt_response field from gemini-formatted JSON output.
func TestNormalizeOutput_Gemini(t *testing.T) {
	t.Parallel()

	// Given: gemini-formatted output
	input := `{"prompt_response": "gemini answer"}`

	// When: NormalizeOutput is called
	got := pipeline.NormalizeOutput("gemini", input)

	// Then: prompt_response is extracted
	assert.Equal(t, "gemini answer", got)
}

// TestNormalizeOutput_PlainText verifies that NormalizeOutput returns the raw
// string unchanged when the output is not structured JSON.
func TestNormalizeOutput_PlainText(t *testing.T) {
	t.Parallel()

	// Given: plain text output (not JSON)
	input := "this is plain text output"

	// When: NormalizeOutput is called with any platform
	got := pipeline.NormalizeOutput("claude", input)

	// Then: the raw text is returned unchanged
	assert.Equal(t, input, got)
}
