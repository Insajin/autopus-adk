package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeFindingKey_IgnoresCaseWhitespaceAndPunctuation(t *testing.T) {
	t.Parallel()

	base := NormalizeFindingKey(ReviewFinding{Description: "Missing REQ-001 coverage"})

	assert.Equal(t, "missing req 001 coverage", base)
	assert.Equal(t, base, NormalizeFindingKey(ReviewFinding{
		Description: "  missing   req-001, coverage.  ",
	}))
	assert.Equal(t, base, NormalizeFindingKey(ReviewFinding{
		Description: "MISSING\tREQ-001\ncoverage!",
	}))
	assert.NotEqual(t, base, NormalizeFindingKey(ReviewFinding{
		Description: "Missing REQ-002 coverage",
	}))
	assert.Empty(t, NormalizeFindingKey(ReviewFinding{Description: " -- .. "}))
}

func TestFindingLocationKey_OnlyFileLineCoordinatesQualify(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "auth/login.go:42",
		FindingLocationKey(ReviewFinding{ScopeRef: "Auth/Login.go:42"}))
	assert.Equal(t, "auth/login.go:42",
		FindingLocationKey(ReviewFinding{ScopeRef: "./auth/login.go:42:9"}))
	assert.Empty(t, FindingLocationKey(ReviewFinding{ScopeRef: "auth/login.go"}),
		"a bare file is too coarse: unrelated findings share a file")
	assert.Empty(t, FindingLocationKey(ReviewFinding{ScopeRef: "REQ-001"}))
	assert.Empty(t, FindingLocationKey(ReviewFinding{ScopeRef: "auth/login.go:head"}))
	assert.Empty(t, FindingLocationKey(ReviewFinding{}))
}

func TestDetectRepeatDiscoveries_MatchesTitleAndLocationButNotTrackedIDs(t *testing.T) {
	t.Parallel()

	prior := []ReviewFinding{
		{ID: "F-001", Status: FindingStatusOpen, ScopeRef: "REQ-001", Description: "Missing REQ-001 coverage"},
		{ID: "F-002", Status: FindingStatusResolved, ScopeRef: "auth/login.go:42", Description: "Token never expires"},
	}
	incoming := []ReviewFinding{
		{ID: "F-001", Status: FindingStatusOpen, ScopeRef: "REQ-001", Description: "Missing REQ-001 coverage"},
		{ID: "F-003", Severity: "major", ScopeRef: "REQ-001", Description: "missing req-001 coverage."},
		{ID: "F-004", Severity: "major", ScopeRef: "auth/login.go:42", Description: "Session lifetime is unbounded"},
		{ID: "F-005", Severity: "major", ScopeRef: "REQ-009", Description: "Rollback plan is absent"},
	}

	repeats := DetectRepeatDiscoveries(prior, incoming, 2)

	require.Len(t, repeats, 2)
	assert.Equal(t, RepeatDiscovery{FindingID: "F-003", MatchedPriorID: "F-001", Revision: 2}, repeats[0])
	assert.Equal(t, RepeatDiscovery{FindingID: "F-004", MatchedPriorID: "F-002", Revision: 2}, repeats[1])
}

func TestApplyRepeatDiscoveries_ScopesOutRepeatsAndLeavesNewFindingsToScopeLock(t *testing.T) {
	t.Parallel()

	prior := []ReviewFinding{
		{ID: "F-001", Status: FindingStatusOpen, ScopeRef: "REQ-001", Description: "Missing REQ-001 coverage"},
	}
	incoming := []ReviewFinding{
		{ID: "F-002", Severity: "major", ScopeRef: "REQ-001", Description: "Missing REQ-001 coverage!"},
		{ID: "F-003", Severity: "major", ScopeRef: "REQ-009", Description: "Rollback plan is absent"},
	}

	classified, repeats := ApplyRepeatDiscoveries(incoming, prior, 1)

	require.Len(t, classified, 2)
	assert.Equal(t, FindingStatusOutOfScope, classified[0].Status)
	assert.Equal(t, "F-001", classified[0].RepeatOf)
	assert.False(t, classified[0].EscapeHatch)
	assert.Empty(t, classified[1].RepeatOf, "an unmatched finding stays new")
	assert.Empty(t, string(classified[1].Status))
	require.Len(t, repeats, 1)
	assert.Equal(t, "F-002", repeats[0].FindingID)

	locked := ApplyScopeLock(classified, prior, ReviewModeVerify)
	require.Len(t, locked, 2)
	assert.Equal(t, FindingStatusOutOfScope, locked[0].Status, "scope lock must not overwrite a repeat verdict")
	assert.Equal(t, "F-001", locked[0].RepeatOf)
	assert.Equal(t, FindingStatusOutOfScope, locked[1].Status, "unmatched noncritical finding is scope-locked as before")
	assert.Empty(t, locked[1].RepeatOf)
}

func TestApplyRepeatDiscoveries_CriticalRepeatOfResolvedFindingRegresses(t *testing.T) {
	t.Parallel()

	prior := []ReviewFinding{
		{ID: "F-001", Status: FindingStatusResolved, Severity: "critical",
			ScopeRef: "auth/login.go:42", Description: "SQL injection in login handler"},
		{ID: "F-002", Status: FindingStatusOpen, Severity: "critical",
			ScopeRef: "auth/token.go:7", Description: "Token signature never verified"},
	}
	incoming := []ReviewFinding{
		{ID: "F-003", Severity: "critical", Category: FindingCategorySecurity,
			ScopeRef: "auth/login.go:42", Description: "sql injection in login handler"},
		{ID: "F-004", Severity: "critical", Category: FindingCategorySecurity,
			ScopeRef: "auth/token.go:7", Description: "Token signature never verified"},
	}

	classified, repeats := ApplyRepeatDiscoveries(incoming, prior, 3)

	require.Len(t, classified, 2)
	assert.Equal(t, FindingStatusRegressed, classified[0].Status)
	assert.True(t, classified[0].EscapeHatch, "a critical repeat keeps its escape hatch")
	assert.True(t, IsActiveBlockingFinding(classified[0]))
	assert.Equal(t, "F-001", classified[0].RepeatOf)
	assert.Equal(t, FindingStatusOpen, classified[1].Status, "repeat of a still-open critical stays open")
	assert.True(t, classified[1].EscapeHatch)
	assert.Len(t, repeats, 2)

	assert.Equal(t, FindingStatusResolved, prior[0].Status, "the resolved prior finding is never re-opened")
}

func TestMergeRepeatDiscoveries_DropsDuplicateProviderRestatements(t *testing.T) {
	t.Parallel()

	first := []RepeatDiscovery{
		{FindingID: "F-002", MatchedPriorID: "F-001", Revision: 1},
		{FindingID: "F-003", MatchedPriorID: "F-001", Revision: 1},
	}
	second := []RepeatDiscovery{
		{FindingID: "F-002", MatchedPriorID: "F-001", Revision: 1},
		{FindingID: "F-002", MatchedPriorID: "F-001", Revision: 2},
	}

	merged := MergeRepeatDiscoveries(first, second)

	require.Len(t, merged, 3)
	assert.Equal(t, "F-002", merged[0].FindingID)
	assert.Equal(t, "F-003", merged[1].FindingID)
	assert.Equal(t, 2, merged[2].Revision, "the same restatement in a later revision is a new signal")
}

func TestRepeatDiscoverySummary_PrintsOnlyWhenDetected(t *testing.T) {
	t.Parallel()

	assert.Empty(t, RepeatDiscoverySummary(0, false))
	assert.Equal(t, "repeat discoveries: 0 (same input: yes)", RepeatDiscoverySummary(0, true))
	assert.Equal(t, "repeat discoveries: 2 (same input: no)", RepeatDiscoverySummary(2, false))
	assert.Equal(t, "repeat discoveries: 2 (same input: yes)", RepeatDiscoverySummary(2, true))
}
