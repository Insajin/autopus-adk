package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatReviewMd_RendersJudgeSummary(t *testing.T) {
	t.Parallel()

	result := &ReviewResult{
		SpecID:  "SPEC-OMP-006",
		Verdict: VerdictPass,
		Judge: &JudgeSummary{
			Provider: "claude",
			Family:   "anthropic",
			Status:   "ok",
			Verdict:  "REVISE",
			Accepted: 1,
			Rejected: 1,
			Merged:   1,
		},
	}

	body := formatReviewMd(result)

	assert.Contains(t, body, "## Judge\n\n")
	assert.Contains(t, body, "**Provider**: claude\n")
	assert.Contains(t, body, "**Family**: anthropic\n")
	assert.Contains(t, body, "**Status**: ok\n")
	assert.Contains(t, body, "**Verdict**: REVISE\n")
	assert.Contains(t, body, "**Accepted**: 1\n")
	assert.Contains(t, body, "**Rejected**: 1\n")
	assert.Contains(t, body, "**Merged**: 1\n")
	assert.NotContains(t, body, "**Reason**:")
	assert.NotContains(t, body, "Rationale")
}

func TestRenderJudgeSection_FailureIncludesReason(t *testing.T) {
	t.Parallel()

	body := renderJudgeSection(&JudgeSummary{
		Provider: "claude", Status: "invalid", Reason: "invalid review judge JSON",
	})

	assert.Contains(t, body, "**Reason**: invalid review judge JSON\n")
	assert.NotContains(t, body, "Rationale")
}

func TestFormatReviewMd_NoJudgeKeepsSectionAbsent(t *testing.T) {
	t.Parallel()

	body := formatReviewMd(&ReviewResult{SpecID: "SPEC-OMP-006", Verdict: VerdictPass})

	assert.NotContains(t, body, "## Judge")
}
