package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVerdict_ParsesChecklistOutcomes(t *testing.T) {
	t.Parallel()

	output := `VERDICT: REVISE
CHECKLIST: Q-CORR-01 | PASS
CHECKLIST: Q-COMP-03 | FAIL | "acceptance.md에 error path 시나리오 부재"
FINDING: [major] [completeness] autopus-adk/.autopus/specs/SPEC-X-001/acceptance.md Error path 미커버`

	result := ParseVerdict("SPEC-X-001", output, "claude", 0, nil)

	require.Len(t, result.ChecklistOutcomes, 2)
	assert.Equal(t, ChecklistOutcome{
		ID:       "Q-CORR-01",
		Status:   ChecklistStatusPass,
		Provider: "claude",
		Revision: 0,
	}, result.ChecklistOutcomes[0])
	assert.Equal(t, ChecklistOutcome{
		ID:       "Q-COMP-03",
		Status:   ChecklistStatusFail,
		Reason:   "acceptance.md에 error path 시나리오 부재",
		Provider: "claude",
		Revision: 0,
	}, result.ChecklistOutcomes[1])
	assert.Equal(t, VerdictRevise, result.Verdict)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, FindingCategoryCompleteness, result.Findings[0].Category)
}

// S1 (SPEC-SPECREV-002 REQ-001): an N/A checklist line is parsed without loss,
// preserving its reason text.
func TestParseVerdict_ParsesNAChecklistOutcome(t *testing.T) {
	t.Parallel()

	output := "VERDICT: PASS\n" +
		"CHECKLIST: Q-SEC-01 | N/A | doc-only SPEC, no trust boundary"

	result := ParseVerdict("SPEC-X-001", output, "claude", 0, nil)

	require.Len(t, result.ChecklistOutcomes, 1)
	assert.Equal(t, ChecklistOutcome{
		ID:       "Q-SEC-01",
		Status:   ChecklistStatusNA,
		Reason:   "doc-only SPEC, no trust boundary",
		Provider: "claude",
		Revision: 0,
	}, result.ChecklistOutcomes[0])
}

// S2 (SPEC-SPECREV-002 REQ-001): PASS, FAIL, and N/A are all parsed and counted
// from a single output.
func TestParseVerdict_CountsPassFailNA(t *testing.T) {
	t.Parallel()

	output := "VERDICT: REVISE\n" +
		"CHECKLIST: Q-CORR-01 | PASS\n" +
		"CHECKLIST: Q-COMP-03 | FAIL | \"근거\"\n" +
		"CHECKLIST: Q-SEC-01 | N/A | \"doc-only\""

	result := ParseVerdict("SPEC-X-001", output, "claude", 0, nil)

	require.Len(t, result.ChecklistOutcomes, 3)
	pass, fail, na := CountChecklistStatuses(result.ChecklistOutcomes)
	assert.Equal(t, 1, pass)
	assert.Equal(t, 1, fail)
	assert.Equal(t, 1, na)
}
