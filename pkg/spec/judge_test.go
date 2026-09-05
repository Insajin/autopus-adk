package spec

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJudgedFindingsToReview_AcceptsOnlyAndRestoresSources(t *testing.T) {
	t.Parallel()

	out := &orchestra.ReviewJudgeOutput{Findings: []orchestra.JudgedFinding{
		{
			ID:          "F-009",
			Severity:    "MAJOR",
			Category:    "correctness",
			ScopeRef:    "REQ-JUDGE-001",
			Location:    "spec.md:54",
			Description: "The judge stage is not wired.",
			Suggestion:  "Invoke the judge after reviewers complete.",
			Decision:    "accept",
			Sources:     []string{"Reviewer A", "Reviewer B"},
		},
		{
			Severity: "minor", Category: "style", Location: "plan.md:40",
			Description: "Duplicate wording.", Suggestion: "Remove it.",
			Decision: "reject", Sources: []string{"Reviewer A"}, Reason: "Not actionable.",
		},
		{
			ID: "F-010", MergeInto: "F-009", Severity: "major", Category: "completeness", Location: "acceptance.md:46",
			Description: "Overlaps the accepted finding.", Suggestion: "Merge it.",
			Decision: "merge", Sources: []string{"Reviewer C"}, Reason: "Same root cause.",
		},
		{
			Severity: "minor", Category: "feasibility", Location: "research.md:10",
			Description: "A second accepted issue.", Suggestion: "Clarify the constraint.",
			Decision: "accept", Sources: []string{"Reviewer B", "Reviewer Z"},
		},
	}}

	got := JudgedFindingsToReview("SPEC-OMP-006", 2, out, map[string]string{
		"Reviewer A": "claude",
		"Reviewer B": "codex",
		"Reviewer C": "gemini",
	}, nil)

	require.Len(t, got, 2)
	assert.Equal(t, "F-009", got[0].ID)
	// The merge finding that names F-009 contributes its source only.
	assert.Equal(t, "claude, codex, gemini", got[0].Provider)
	assert.Equal(t, "major", got[0].Severity)
	assert.Equal(t, FindingCategoryCorrectness, got[0].Category)
	assert.Equal(t, "REQ-JUDGE-001", got[0].ScopeRef)
	assert.Equal(t, FindingStatusOpen, got[0].Status)
	assert.Equal(t, 2, got[0].FirstSeenRev)
	assert.Equal(t, 2, got[0].LastSeenRev)

	assert.Equal(t, "F-001", got[1].ID)
	assert.Equal(t, "codex, Reviewer Z", got[1].Provider)
	assert.Equal(t, "research.md:10", got[1].ScopeRef)
}

func TestJudgedFindingsToReview_NilOutputIsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, JudgedFindingsToReview("SPEC-OMP-006", 0, nil, nil, nil))
}

func TestJudgedFindingsToReview_VerifyPreservesPriorLifecycleAndNumbersNewFindings(t *testing.T) {
	t.Parallel()

	prior := []ReviewFinding{
		{
			ID: "F-001", Provider: "claude", Severity: "major",
			Category: FindingCategoryCorrectness, ScopeRef: "REQ-001",
			Description: "Existing issue", Status: FindingStatusOpen,
			FirstSeenRev: 1, LastSeenRev: 1,
		},
		{
			ID: "F-002", Provider: "codex", Severity: "minor",
			Category: FindingCategoryCompleteness, ScopeRef: "REQ-002",
			Description: "Issue fixed this round", Status: FindingStatusOpen,
			FirstSeenRev: 1, LastSeenRev: 1,
		},
	}
	out := &orchestra.ReviewJudgeOutput{Findings: []orchestra.JudgedFinding{
		{
			ID: "F-001", Severity: "critical", Category: "correctness",
			Description: "Reworded existing issue", Decision: "accept",
			Sources: []string{"Reviewer B"},
		},
		{
			ID: "F-999", Severity: "minor", Category: "feasibility",
			Location: "plan.md:20", Description: "New issue", Decision: "accept",
			Sources: []string{"Reviewer B"},
		},
	}}

	got := JudgedFindingsToReview(
		"SPEC-OMP-006", 2, out, map[string]string{"Reviewer B": "gemini"}, prior,
	)

	require.Len(t, got, 3)
	byID := make(map[string]ReviewFinding, len(got))
	for _, finding := range got {
		byID[finding.ID] = finding
	}
	assert.Equal(t, FindingStatusOpen, byID["F-001"].Status)
	assert.Equal(t, 1, byID["F-001"].FirstSeenRev)
	assert.Equal(t, 2, byID["F-001"].LastSeenRev)
	assert.Equal(t, "claude, gemini", byID["F-001"].Provider, "prior attribution first, judge sources appended")
	assert.Equal(t, "Reworded existing issue", byID["F-001"].Description)
	assert.Equal(t, "critical", byID["F-001"].Severity)
	assert.Equal(t, "REQ-001", byID["F-001"].ScopeRef)
	assert.Equal(t, FindingStatusResolved, byID["F-002"].Status)
	assert.Equal(t, 1, byID["F-002"].FirstSeenRev)
	assert.Equal(t, 2, byID["F-002"].LastSeenRev)
	assert.Equal(t, "codex", byID["F-002"].Provider)
	assert.Equal(t, "New issue", byID["F-003"].Description)
	assert.Equal(t, "gemini", byID["F-003"].Provider)
	assert.Equal(t, 2, byID["F-003"].FirstSeenRev)
	assert.NotContains(t, byID, "F-999")
}

// A prior finding the judge had resolved and now accepts again is regressed,
// not a fresh open finding; an already regressed prior stays regressed.
func TestJudgedFindingsToReview_VerifyReacceptedResolvedPriorIsRegressed(t *testing.T) {
	prior := []ReviewFinding{
		{ID: "F-001", Provider: "claude", Severity: "major", Status: FindingStatusResolved, FirstSeenRev: 0, LastSeenRev: 1},
		{ID: "F-002", Provider: "codex", Severity: "minor", Status: FindingStatusRegressed, FirstSeenRev: 0, LastSeenRev: 1},
		{ID: "F-003", Provider: "gemini", Severity: "minor", Status: FindingStatusOpen, FirstSeenRev: 1, LastSeenRev: 1},
	}
	out := &orchestra.ReviewJudgeOutput{Verdict: "REVISE", Findings: []orchestra.JudgedFinding{
		{ID: "F-001", Severity: "major", Location: "spec.md:1", Description: "back", Decision: "accept", Sources: []string{"Reviewer A"}},
		{ID: "F-002", Severity: "minor", Location: "spec.md:2", Description: "still", Decision: "accept", Sources: []string{"Reviewer A"}},
		{ID: "F-003", Severity: "minor", Location: "spec.md:3", Description: "open", Decision: "accept", Sources: []string{"Reviewer A"}},
	}}

	got := JudgedFindingsToReview("SPEC-OMP-006", 2, out, map[string]string{"Reviewer A": "claude"}, prior)

	byID := make(map[string]ReviewFinding, len(got))
	for _, finding := range got {
		byID[finding.ID] = finding
	}
	assert.Equal(t, FindingStatusRegressed, byID["F-001"].Status)
	assert.Equal(t, FindingStatusRegressed, byID["F-002"].Status)
	assert.Equal(t, FindingStatusOpen, byID["F-003"].Status)
	assert.Equal(t, 0, byID["F-001"].FirstSeenRev)
}
