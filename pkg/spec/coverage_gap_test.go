package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for buildVerifyInstructions (prompt.go) — was 0% covered

func TestBuildReviewPrompt_VerifyMode_ContainsChecklist(t *testing.T) {
	t.Parallel()

	doc := &SpecDocument{
		ID:    "SPEC-AUTH-001",
		Title: "Auth",
	}
	priorFindings := []ReviewFinding{
		{ID: "F-001", Severity: "major", ScopeRef: "REQ-001", Description: "Missing error case", Status: FindingStatusOpen},
		{ID: "F-002", Severity: "minor", ScopeRef: "types.go:10", Description: "Naming issue", Status: FindingStatusOpen},
	}
	opts := ReviewPromptOptions{
		Mode:          ReviewModeVerify,
		PriorFindings: priorFindings,
	}

	prompt := BuildReviewPrompt(doc, "", opts)

	assert.Contains(t, prompt, "Verify Mode", "verify mode prompt must contain Verify Mode header")
	assert.Contains(t, prompt, "F-001", "prior finding ID must appear in checklist")
	assert.Contains(t, prompt, "F-002", "second finding ID must appear in checklist")
	assert.Contains(t, prompt, "FINDING_STATUS", "verify mode must instruct provider to report FINDING_STATUS")
	assert.Contains(t, prompt, "REQ-001", "ScopeRef must appear in checklist")
}

func TestBuildReviewPrompt_VerifyMode_EmptyPriorFindings(t *testing.T) {
	t.Parallel()

	doc := &SpecDocument{ID: "SPEC-X-001", Title: "Test"}
	opts := ReviewPromptOptions{
		Mode:          ReviewModeVerify,
		PriorFindings: []ReviewFinding{},
	}

	prompt := BuildReviewPrompt(doc, "", opts)

	assert.Contains(t, prompt, "Verify Mode")
	// No findings checklist section expected
	assert.NotContains(t, prompt, "Prior Findings Checklist")
}

func TestBuildReviewPrompt_VerifyMode_ViaExplicitMode(t *testing.T) {
	t.Parallel()

	// opts.Mode == ReviewModeVerify with nil PriorFindings still enters verify path
	doc := &SpecDocument{ID: "SPEC-Y-001", Title: "Y"}
	opts := ReviewPromptOptions{Mode: ReviewModeVerify}

	prompt := BuildReviewPrompt(doc, "", opts)

	assert.Contains(t, prompt, "Verify Mode")
}

func TestBuildReviewPrompt_DiscoverMode_WithStaticFindings(t *testing.T) {
	t.Parallel()

	doc := &SpecDocument{ID: "SPEC-Z-001", Title: "Z"}
	staticFindings := []ReviewFinding{
		{Category: FindingCategoryStyle, Severity: "minor", ScopeRef: "pkg/foo.go:5", Description: "revive: exported func missing comment"},
	}
	opts := ReviewPromptOptions{
		Mode:           ReviewModeDiscover,
		StaticFindings: staticFindings,
	}

	prompt := BuildReviewPrompt(doc, "", opts)

	assert.Contains(t, prompt, "Already Discovered Static Analysis Issues")
	assert.Contains(t, prompt, "pkg/foo.go:5")
	assert.NotContains(t, prompt, "Verify Mode")
}

func TestBuildReviewPrompt_DiscoverMode_NoStaticFindings(t *testing.T) {
	t.Parallel()

	doc := &SpecDocument{ID: "SPEC-A-001", Title: "A"}
	opts := ReviewPromptOptions{Mode: ReviewModeDiscover}

	prompt := BuildReviewPrompt(doc, "", opts)

	assert.NotContains(t, prompt, "Already Discovered Static Analysis Issues")
	assert.Contains(t, prompt, "VERDICT")
}

// Tests for parseVerifyFindings (reviewer.go) — was 0% covered

func TestParseVerdict_VerifyMode_UpdatesStatusResolved(t *testing.T) {
	t.Parallel()

	prior := []ReviewFinding{
		{ID: "F-001", Status: FindingStatusOpen, Category: FindingCategoryCorrectness, Description: "Bug"},
	}
	output := `VERDICT: PASS
FINDING_STATUS: F-001 | resolved | Fixed in latest revision`

	result := ParseVerdict("SPEC-X-001", output, "claude", 1, prior)

	require.Len(t, result.Findings, 1)
	assert.Equal(t, FindingStatusResolved, result.Findings[0].Status)
	assert.Equal(t, VerdictPass, result.Verdict)
}

func TestParseVerdict_VerifyMode_UpdatesStatusRegressed(t *testing.T) {
	t.Parallel()

	prior := []ReviewFinding{
		{ID: "F-001", Status: FindingStatusResolved, Category: FindingCategoryCorrectness, Description: "Was fixed"},
	}
	output := `VERDICT: REVISE
FINDING_STATUS: F-001 | regressed | Broke again in new code`

	result := ParseVerdict("SPEC-X-001", output, "claude", 2, prior)

	require.Len(t, result.Findings, 1)
	assert.Equal(t, FindingStatusRegressed, result.Findings[0].Status)
}

func TestParseVerdict_VerifyMode_NewCriticalFindingEscapeHatch(t *testing.T) {
	t.Parallel()

	prior := []ReviewFinding{
		{ID: "F-001", Status: FindingStatusOpen, Category: FindingCategoryCorrectness, Description: "Existing"},
	}
	output := `VERDICT: REVISE
FINDING_STATUS: F-001 | open | still open
FINDING: [critical] [security] auth.go:42 SQL injection in login handler`

	result := ParseVerdict("SPEC-AUTH-001", output, "claude", 2, prior)

	// F-001 stays open; new finding F-002 is escape hatch
	require.Len(t, result.Findings, 2)
	var escapedFindings []ReviewFinding
	for _, f := range result.Findings {
		if f.EscapeHatch {
			escapedFindings = append(escapedFindings, f)
		}
	}
	require.Len(t, escapedFindings, 1)
	assert.Equal(t, FindingStatusOpen, escapedFindings[0].Status)
}

func TestParseVerdict_VerifyMode_NewNonCriticalTaggedOutOfScope(t *testing.T) {
	t.Parallel()

	prior := []ReviewFinding{
		{ID: "F-001", Status: FindingStatusOpen, Category: FindingCategoryCorrectness, Description: "Existing"},
	}
	output := `VERDICT: REVISE
FINDING_STATUS: F-001 | open | still open
FINDING: [minor] [style] types.go:10 naming issue`

	result := ParseVerdict("SPEC-X-001", output, "claude", 2, prior)

	require.Len(t, result.Findings, 2)
	var outOfScope []ReviewFinding
	for _, f := range result.Findings {
		if f.Status == FindingStatusOutOfScope {
			outOfScope = append(outOfScope, f)
		}
	}
	assert.Len(t, outOfScope, 1, "non-critical new finding in verify mode must be out_of_scope")
}

func TestParseVerdict_VerifyMode_LastSeenRevUpdated(t *testing.T) {
	t.Parallel()

	prior := []ReviewFinding{
		{ID: "F-001", Status: FindingStatusOpen, FirstSeenRev: 0, LastSeenRev: 0},
	}
	output := `VERDICT: REVISE
FINDING_STATUS: F-001 | open | unchanged`

	result := ParseVerdict("SPEC-X-001", output, "claude", 3, prior)

	require.Len(t, result.Findings, 1)
	assert.Equal(t, 3, result.Findings[0].LastSeenRev, "LastSeenRev must be updated to current revision")
}

func TestParseVerdict_VerifyMode_PassWithoutStatusesResolvesPriorFindings(t *testing.T) {
	t.Parallel()

	prior := []ReviewFinding{
		{ID: "F-001", Status: FindingStatusOpen, Description: "Existing"},
		{ID: "F-002", Status: FindingStatusRegressed, Description: "Regressed"},
	}
	output := `VERDICT: PASS`

	result := ParseVerdict("SPEC-X-001", output, "claude", 2, prior)

	require.Len(t, result.Findings, 2)
	assert.Equal(t, FindingStatusResolved, result.Findings[0].Status)
	assert.Equal(t, FindingStatusResolved, result.Findings[1].Status)
}
