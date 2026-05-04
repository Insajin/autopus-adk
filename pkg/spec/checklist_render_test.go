package spec_test

// SPEC-SPECREV-001 follow-up: pin the review.md ## Checklist Summary section
// rendering and the per-status counter that drives both the markdown table
// and the stdout summary.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestCountChecklistStatuses_PassFailNATotals(t *testing.T) {
	t.Parallel()

	outcomes := []spec.ChecklistOutcome{
		{ID: "Q-CORR-01", Status: spec.ChecklistStatusPass, Provider: "claude"},
		{ID: "Q-CORR-02", Status: spec.ChecklistStatusFail, Provider: "claude", Reason: "missing"},
		{ID: "Q-SEC-01", Status: spec.ChecklistStatusNA, Provider: "claude", Reason: "doc-only"},
		{ID: "Q-SEC-02", Status: spec.ChecklistStatusNA, Provider: "gemini", Reason: "no trust boundary"},
		{ID: "Q-STYLE-01", Status: spec.ChecklistStatusPass, Provider: "gemini"},
	}

	pass, fail, na := spec.CountChecklistStatuses(outcomes)

	require.Equal(t, 2, pass, "PASS count")
	require.Equal(t, 1, fail, "FAIL count")
	require.Equal(t, 2, na, "N/A count")
}

func TestCountChecklistStatuses_EmptyReturnsZeros(t *testing.T) {
	t.Parallel()

	pass, fail, na := spec.CountChecklistStatuses(nil)
	assert.Zero(t, pass)
	assert.Zero(t, fail)
	assert.Zero(t, na)
}

func TestRenderChecklistSection_EmptyOutcomesProduceNoSection(t *testing.T) {
	t.Parallel()

	got := spec.RenderChecklistSection(nil)
	assert.Empty(t, got, "empty outcomes must omit the section entirely")
}

func TestRenderChecklistSection_HeadingTableAndTotals(t *testing.T) {
	t.Parallel()

	outcomes := []spec.ChecklistOutcome{
		{ID: "Q-CORR-01", Status: spec.ChecklistStatusPass, Provider: "claude", Reason: ""},
		{ID: "Q-COMP-03", Status: spec.ChecklistStatusFail, Provider: "claude", Reason: "missing oracle"},
		{ID: "Q-SEC-01", Status: spec.ChecklistStatusNA, Provider: "gemini", Reason: "doc-only SPEC"},
	}

	got := spec.RenderChecklistSection(outcomes)

	// Heading and column order are pinned so external tooling and reviewers
	// can rely on a stable contract.
	assert.Contains(t, got, "## Checklist Summary")
	assert.Contains(t, got, "| ID | Status | Provider | Reason |")
	assert.Contains(t, got, "| --- | --- | --- | --- |")

	// Each outcome renders one row with status verbatim (PASS/FAIL/N/A).
	assert.Contains(t, got, "| Q-CORR-01 | PASS | claude | - |")
	assert.Contains(t, got, "| Q-COMP-03 | FAIL | claude | missing oracle |")
	assert.Contains(t, got, "| Q-SEC-01 | N/A | gemini | doc-only SPEC |")

	// Totals line mirrors the stdout summary: PASS / FAIL / N/A counts.
	assert.Contains(t, got, "Total: 3 (PASS: 1, FAIL: 1, N/A: 1)")
}

func TestRenderChecklistSection_ReasonSanitization(t *testing.T) {
	t.Parallel()

	// Pipe character in the Reason must not break the markdown column
	// boundary; control characters must collapse to spaces.
	outcomes := []spec.ChecklistOutcome{
		{ID: "Q-X-01", Status: spec.ChecklistStatusFail, Provider: "p", Reason: "a|b\nc"},
	}
	got := spec.RenderChecklistSection(outcomes)

	assert.Contains(t, got, "| Q-X-01 | FAIL | p | a/b c |", "pipe → /, newline → space")
	// The rendered table line must still contain exactly one trailing pipe.
	rowLine := ""
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "Q-X-01") {
			rowLine = line
			break
		}
	}
	require.NotEmpty(t, rowLine)
	assert.Equal(t, 5, strings.Count(rowLine, "|"), "exactly 5 pipe separators per 4-column row")
}
