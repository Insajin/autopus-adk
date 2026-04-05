package pipeline_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// TestEvaluateGate_Validation_PassOnSuccess verifies that "PASS" in output
// yields VerdictPass for a validation gate.
func TestEvaluateGate_Validation_PassOnSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
	}{
		{"uppercase PASS", "All tests passed. PASS"},
		{"PASS at start", "PASS: all validations succeeded"},
		{"lowercase pass ignored", "pass"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// When: EvaluateGate is called for a validation gate
			verdict := pipeline.EvaluateGate(pipeline.GateValidation, tt.output)

			// Then: verdict is Pass when output contains uppercase PASS
			if tt.name == "lowercase pass ignored" {
				assert.Equal(t, pipeline.VerdictFail, verdict)
			} else {
				assert.Equal(t, pipeline.VerdictPass, verdict)
			}
		})
	}
}

// TestEvaluateGate_Validation_FailOnError verifies that "FAIL" in output
// yields VerdictFail for a validation gate.
func TestEvaluateGate_Validation_FailOnError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
	}{
		{"explicit FAIL", "FAIL: coverage below threshold"},
		{"FAIL in middle", "Some passed but FAIL overall"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// When: EvaluateGate is called
			verdict := pipeline.EvaluateGate(pipeline.GateValidation, tt.output)

			// Then: verdict is Fail
			assert.Equal(t, pipeline.VerdictFail, verdict)
		})
	}
}

// TestEvaluateGate_Review_ApproveOnApproval verifies that "APPROVE" in output
// yields VerdictPass for a review gate.
func TestEvaluateGate_Review_ApproveOnApproval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
	}{
		{"explicit APPROVE", "APPROVE: changes look good"},
		{"APPROVED variant", "APPROVED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// When: EvaluateGate is called for a review gate
			verdict := pipeline.EvaluateGate(pipeline.GateReview, tt.output)

			// Then: verdict is Pass
			assert.Equal(t, pipeline.VerdictPass, verdict)
		})
	}
}

// TestEvaluateGate_Review_RequestChanges verifies that "REQUEST_CHANGES" in
// output yields VerdictFail for a review gate.
func TestEvaluateGate_Review_RequestChanges(t *testing.T) {
	t.Parallel()

	// Given: review output requesting changes
	output := "REQUEST_CHANGES: fix the error handling"

	// When: EvaluateGate is called
	verdict := pipeline.EvaluateGate(pipeline.GateReview, output)

	// Then: verdict is Fail
	assert.Equal(t, pipeline.VerdictFail, verdict)
}

// TestEvaluateGate_None_AlwaysPass verifies that GateNone always yields
// VerdictPass regardless of output content.
func TestEvaluateGate_None_AlwaysPass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
	}{
		{"empty output", ""},
		{"fail-like output", "FAIL: something wrong"},
		{"random content", "some random content"},
		{"request changes", "REQUEST_CHANGES"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// When: EvaluateGate is called with GateNone
			verdict := pipeline.EvaluateGate(pipeline.GateNone, tt.output)

			// Then: verdict is always Pass
			assert.Equal(t, pipeline.VerdictPass, verdict)
		})
	}
}
