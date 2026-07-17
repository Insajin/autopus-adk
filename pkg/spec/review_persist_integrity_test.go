package spec

// Oracle tests for SPEC-ADK-REVIEW-INTEGRITY-001 Task B: review.md observation
// rendering. These pin the render half of REQ-RINT-COV-01 (Observation Coverage
// section), REQ-RINT-TRUNC-04 / REQ-RINT-QUORUM-05 (verdict degraded reasons),
// and REQ-RINT-OVERRIDE-07 (override audit line). Each test asserts concrete
// markdown substrings so the rendered contract is observable, and the absence
// cases pin byte-stability versus the pre-integrity output.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFormatReviewMd_ObservationCoverageSection pins the AC-RINT-COV-1 render:
// a populated DocCoverages set renders a `## Observation Coverage` table whose
// rows carry the concrete injected/total/percent/complete values.
func TestFormatReviewMd_ObservationCoverageSection(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{
		SpecID:  "SPEC-COV-001",
		Verdict: VerdictPass,
		DocCoverages: []DocCoverage{
			{Name: "research.md", Injected: 200, Total: 250, Percent: 80, Complete: false},
			{Name: "plan.md", Injected: 150, Total: 150, Percent: 100, Complete: true},
		},
	}

	out := formatReviewMd(r)

	assert.Contains(t, out, "## Observation Coverage",
		"coverage section header must render when DocCoverages is populated")
	assert.Contains(t, out, "| research.md | 200 | 250 | 80% | no |",
		"partial-coverage row must render concrete injected/total/percent")
	assert.Contains(t, out, "| plan.md | 150 | 150 | 100% | yes |",
		"complete row must render 100%% coverage and yes")
}

// TestFormatReviewMd_EmptyCoverageOmitsSection proves the section is skipped
// cleanly so prior results without coverage render exactly as before.
func TestFormatReviewMd_EmptyCoverageOmitsSection(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{SpecID: "SPEC-COV-EMPTY", Verdict: VerdictPass}

	out := formatReviewMd(r)

	assert.NotContains(t, out, "## Observation Coverage",
		"section must be omitted when DocCoverages is empty (backward-compat)")
}

// TestFormatReviewMd_DegradedReasonsVerdict pins the exact verdict line when both
// observation-integrity degraded reasons are present without any provider-ratio
// degradation (REQ-RINT-TRUNC-04 / REQ-RINT-QUORUM-05 render half).
func TestFormatReviewMd_DegradedReasonsVerdict(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{
		SpecID:          "SPEC-DEG-001",
		Verdict:         VerdictPass,
		DegradedReasons: []string{DegradedReasonPartialDocContext, DegradedReasonProviderQuorum},
	}

	out := formatReviewMd(r)

	want := "**Verdict**: PASS (degraded: partial_doc_context, provider_quorum)"
	assert.Contains(t, out, want,
		"verdict line must carry the joined degraded reasons; got:\n%s", out)
}

// TestFormatReviewMd_SingleDegradedReason verifies a single reason renders without
// a trailing separator.
func TestFormatReviewMd_SingleDegradedReason(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{
		SpecID:          "SPEC-DEG-002",
		Verdict:         VerdictPass,
		DegradedReasons: []string{DegradedReasonPartialDocContext},
	}

	out := formatReviewMd(r)

	assert.Contains(t, out, "**Verdict**: PASS (degraded: partial_doc_context)")
	assert.NotContains(t, out, "partial_doc_context,",
		"single reason must not render a dangling comma")
}

// TestFormatReviewMd_QuorumDegradedBothAxes documents the real sub-quorum PASS
// case where the provider-ratio DegradedLabel and the reason-code suffix co-occur
// on one verdict line. Task E sets DegradedReasons from the same aggregation, so
// this is the exact string integration assertions can pin against.
func TestFormatReviewMd_QuorumDegradedBothAxes(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{
		SpecID:  "SPEC-QUORUM-001",
		Verdict: VerdictPass,
		ProviderStatuses: []ProviderStatus{
			{Provider: "claude", Status: "success", Note: "-"},
			{Provider: "gemini", Status: "timeout", Note: "-"},
			{Provider: "codex", Status: "timeout", Note: "-"},
		},
		DegradedReasons: []string{DegradedReasonProviderQuorum},
	}

	out := formatReviewMd(r)

	want := "**Verdict**: PASS (degraded — 1/3 providers responded) (degraded: provider_quorum)"
	assert.Contains(t, out, want,
		"ratio label and reason-code suffix co-occur deterministically; got:\n%s", out)
}

// TestFormatReviewMd_NilDegradedReasonsVerdictUnchanged pins byte-stability of the
// header block versus today's golden when no integrity fields are set.
func TestFormatReviewMd_NilDegradedReasonsVerdictUnchanged(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{SpecID: "SPEC-DEG-NIL", Verdict: VerdictPass}

	out := formatReviewMd(r)

	assert.True(t, strings.HasPrefix(out,
		"# Review: SPEC-DEG-NIL\n\n**Verdict**: PASS\n**Revision**: 0\n**Date**: "),
		"header block must stay byte-stable when no integrity fields are set; got:\n%s", out)
	assert.NotContains(t, out, "(degraded",
		"no degraded token when DegradedReasons and ProviderStatuses are empty")
}

// TestFormatReviewMd_OverrideAuditLine pins the AC-RINT-OVERRIDE-5 render: an
// override promotion writes a deterministic audit line naming allow-degraded and
// the degraded reason.
func TestFormatReviewMd_OverrideAuditLine(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{
		SpecID:            "SPEC-OVR-001",
		Verdict:           VerdictPass,
		DegradedReasons:   []string{DegradedReasonProviderQuorum},
		OverridePromotion: true,
	}

	out := formatReviewMd(r)

	want := "**Promotion Override**: --allow-degraded accepted by operator despite provider_quorum"
	assert.Contains(t, out, want,
		"override audit line must name allow-degraded and the reason; got:\n%s", out)
}

// TestFormatReviewMd_NoOverrideOmitsAuditLine proves the audit line is absent when
// no override occurred, keeping legacy output stable.
func TestFormatReviewMd_NoOverrideOmitsAuditLine(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{SpecID: "SPEC-OVR-NONE", Verdict: VerdictPass}

	out := formatReviewMd(r)

	assert.NotContains(t, out, "**Promotion Override**",
		"audit line must be omitted when OverridePromotion is false")
	assert.NotContains(t, out, "allow-degraded",
		"no allow-degraded token without an override")
}

// TestFormatReviewMd_OverrideWithoutReasonsFallback verifies a deterministic
// fallback phrase when OverridePromotion is set but no reasons accompany it, so
// the audit line stays diff-stable in that edge case.
func TestFormatReviewMd_OverrideWithoutReasonsFallback(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{
		SpecID:            "SPEC-OVR-002",
		Verdict:           VerdictPass,
		OverridePromotion: true,
	}

	out := formatReviewMd(r)

	assert.Contains(t, out,
		"**Promotion Override**: --allow-degraded accepted by operator despite degraded observation",
		"empty reasons fall back to a stable phrase while still naming allow-degraded")
}
