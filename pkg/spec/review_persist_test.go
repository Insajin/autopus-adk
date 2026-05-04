package spec

// Phase 1.5 test scaffold for SPEC-SPECREV-001 REQ-VERD-1 / REQ-VERD-2 / REQ-VERD-4.
// Exercises formatReviewMd to ensure the verdict line and Provider Health
// section are rendered as documented in acceptance.md.
//
// References ReviewResult.ProviderStatuses and spec.ProviderStatus, which do
// not yet exist — compile failure is the expected RED state.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFormatReviewMd_VerdictWithoutDegradedLabel covers AC-VERD-2: when no
// degradation threshold is met, the verdict line MUST NOT carry the label.
func TestFormatReviewMd_VerdictWithoutDegradedLabel(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{
		SpecID:  "SPEC-FAKE-VERD2",
		Verdict: VerdictPass,
		ProviderStatuses: []ProviderStatus{
			{Provider: "claude", Status: "success", Note: "-"},
			{Provider: "gemini", Status: "success", Note: "-"},
			{Provider: "codex", Status: "timeout", Note: "-"},
		},
	}

	out := formatReviewMd(r)

	assert.Contains(t, out, "**Verdict**: PASS",
		"verdict line must include PASS")
	assert.NotContains(t, out, "(degraded",
		"33%% failure must not trigger degraded label (REQ-VERD-2 boundary)")
	assert.Contains(t, out, "## Provider Health",
		"section is rendered even when not degraded (REQ-VERD-1)")
	assert.Contains(t, out, "| codex | timeout |",
		"timeout row is preserved in the table")
}

// TestFormatReviewMd_VerdictWithDegradedLabel covers AC-VERD-1: 1 PASS / 2 timeout
// in normal mode produces REVISE plus the degraded label "1/3 providers responded".
func TestFormatReviewMd_VerdictWithDegradedLabel(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{
		SpecID:  "SPEC-FAKE-VERD1",
		Verdict: VerdictRevise,
		ProviderStatuses: []ProviderStatus{
			{Provider: "claude", Status: "success", Note: "-"},
			{Provider: "gemini", Status: "timeout", Note: "-"},
			{Provider: "codex", Status: "timeout", Note: "-"},
		},
	}

	out := formatReviewMd(r)

	want := "**Verdict**: REVISE (degraded — 1/3 providers responded)"
	assert.True(t, strings.Contains(out, want),
		"verdict line must exactly read %q; got:\n%s", want, out)

	// AC-VERD-1: all three rows of the Provider Health table.
	assert.Contains(t, out, "| Provider | Status | Note |")
	assert.Contains(t, out, "| claude | success |")
	assert.Contains(t, out, "| gemini | timeout |")
	assert.Contains(t, out, "| codex | timeout |")
}

// TestFormatReviewMd_PassWithDegradedLabel covers AC-VERD-3 / REQ-VERD-4:
// when exclude_failed_from_denom yields PASS while real timeouts exist,
// the verdict line MUST still carry the degraded label so users see the
// infra-failure context.
func TestFormatReviewMd_PassWithDegradedLabel(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{
		SpecID:  "SPEC-FAKE-VERD3",
		Verdict: VerdictPass,
		ProviderStatuses: []ProviderStatus{
			{Provider: "claude", Status: "success", Note: "-"},
			{Provider: "gemini", Status: "timeout", Note: "-"},
			{Provider: "codex", Status: "timeout", Note: "-"},
		},
	}

	out := formatReviewMd(r)

	want := "**Verdict**: PASS (degraded — 1/3 providers responded)"
	assert.True(t, strings.Contains(out, want),
		"PASS-with-degraded label is required by REQ-VERD-4; got:\n%s", out)
}

// TestFormatReviewMd_AllFailedDegradedLabel covers AC-VERD-EMPTY: 0/M responded
// must still produce the degraded label with the zero numerator.
func TestFormatReviewMd_AllFailedDegradedLabel(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{
		SpecID:  "SPEC-FAKE-EMPTY",
		Verdict: VerdictRevise,
		ProviderStatuses: []ProviderStatus{
			{Provider: "claude", Status: "timeout", Note: "-"},
			{Provider: "gemini", Status: "timeout", Note: "-"},
			{Provider: "codex", Status: "timeout", Note: "-"},
		},
	}

	out := formatReviewMd(r)

	want := "**Verdict**: REVISE (degraded — 0/3 providers responded)"
	assert.True(t, strings.Contains(out, want),
		"all-failed branch must render N=0 in degraded label; got:\n%s", out)
}

// TestFormatReviewMd_NoStatusesOmitsHealthSection guards against rendering an
// empty Provider Health section when callers do not populate ProviderStatuses
// (legacy code paths).
func TestFormatReviewMd_NoStatusesOmitsHealthSection(t *testing.T) {
	t.Parallel()

	r := &ReviewResult{
		SpecID:  "SPEC-FAKE-LEGACY",
		Verdict: VerdictPass,
	}

	out := formatReviewMd(r)

	assert.NotContains(t, out, "## Provider Health",
		"section must be omitted when ProviderStatuses is empty (backward-compat)")
	assert.Contains(t, out, "**Verdict**: PASS")
	assert.NotContains(t, out, "(degraded")
}
