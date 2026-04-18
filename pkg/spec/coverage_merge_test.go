package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for MergeFindingStatuses (reviewer.go) — was 0% covered

func TestMergeFindingStatuses_ResolvedSupermajority(t *testing.T) {
	t.Parallel()

	// 2 providers both say resolved → 2/2 = 1.0 >= 0.67 threshold → resolved
	providerResults := [][]ReviewFinding{
		{{ID: "F-001", Status: FindingStatusResolved}},
		{{ID: "F-001", Status: FindingStatusResolved}},
	}

	merged := MergeFindingStatuses(providerResults, 0.67)

	require.Len(t, merged, 1)
	assert.Equal(t, FindingStatusResolved, merged[0].Status)
}

func TestMergeFindingStatuses_BelowThresholdStaysOpen(t *testing.T) {
	t.Parallel()

	// 3 providers: 1 says resolved, 2 say open → 1/3 = 0.33 < 0.67 threshold → stays open
	providerResults := [][]ReviewFinding{
		{{ID: "F-001", Status: FindingStatusResolved}},
		{{ID: "F-001", Status: FindingStatusOpen}},
		{{ID: "F-001", Status: FindingStatusOpen}},
	}

	// Use threshold=0.5 so that 1/3 is clearly below (but 2/3 would be above for resolved test)
	merged := MergeFindingStatuses(providerResults, 0.5)

	require.Len(t, merged, 1)
	// 1/3 = 0.33 < 0.5 → not resolved; no regressed → open
	assert.Equal(t, FindingStatusOpen, merged[0].Status)
}

func TestMergeFindingStatuses_AnyRegressedWins(t *testing.T) {
	t.Parallel()

	// Open + regressed → regressed wins even if resolved not supermajority
	providerResults := [][]ReviewFinding{
		{{ID: "F-001", Status: FindingStatusOpen}},
		{{ID: "F-001", Status: FindingStatusRegressed}},
	}

	merged := MergeFindingStatuses(providerResults, 0.67)

	require.Len(t, merged, 1)
	assert.Equal(t, FindingStatusRegressed, merged[0].Status)
}

func TestMergeFindingStatuses_EmptyInput(t *testing.T) {
	t.Parallel()

	result := MergeFindingStatuses(nil, 0.67)
	assert.Nil(t, result)
}

func TestMergeFindingStatuses_MultipleFindingIDs(t *testing.T) {
	t.Parallel()

	providerResults := [][]ReviewFinding{
		{
			{ID: "F-001", Status: FindingStatusResolved},
			{ID: "F-002", Status: FindingStatusOpen},
		},
		{
			{ID: "F-001", Status: FindingStatusResolved},
			{ID: "F-002", Status: FindingStatusRegressed},
		},
	}

	merged := MergeFindingStatuses(providerResults, 0.67)

	require.Len(t, merged, 2)
	statusByID := make(map[string]FindingStatus)
	for _, f := range merged {
		statusByID[f.ID] = f.Status
	}
	assert.Equal(t, FindingStatusResolved, statusByID["F-001"])
	assert.Equal(t, FindingStatusRegressed, statusByID["F-002"])
}

// Tests for normalizeKeyword (gherkin_parser.go) — was 66.7% covered

func TestNormalizeKeyword_EmptyString(t *testing.T) {
	t.Parallel()

	// Covers the len(kw)==0 early return branch
	got := normalizeKeyword("")
	assert.Equal(t, "", got)
}

func TestNormalizeKeyword_AlreadyCapitalized(t *testing.T) {
	t.Parallel()

	got := normalizeKeyword("Given")
	assert.Equal(t, "Given", got)
}

func TestNormalizeKeyword_AllCaps(t *testing.T) {
	t.Parallel()

	got := normalizeKeyword("WHEN")
	assert.Equal(t, "When", got)
}

// Tests for isSourceFile (prompt.go) — was 80% covered

func TestIsSourceFile_RecognizedExtensions(t *testing.T) {
	t.Parallel()

	recognized := []string{"main.go", "app.py", "index.ts", "script.js", "lib.rs", "Main.java", "helper.rb"}
	for _, name := range recognized {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.True(t, isSourceFile(name), "%s should be recognized as source file", name)
		})
	}
}

func TestIsSourceFile_UnrecognizedExtensions(t *testing.T) {
	t.Parallel()

	unrecognized := []string{"config.yaml", "data.json", "README.md", "archive.zip", "image.png", "noextension"}
	for _, name := range unrecognized {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.False(t, isSourceFile(name), "%s should not be recognized as source file", name)
		})
	}
}

// Tests for isNotFoundError edge case (static_analysis.go) — was 75% covered

func TestIsNotFoundError_StringContainsMessage(t *testing.T) {
	t.Parallel()

	// Covers the strings.Contains fallback branch in isNotFoundError
	findings, err := RunStaticAnalysis(t.TempDir(), "zzz-binary-that-does-not-exist-xyz")
	assert.NoError(t, err, "not-found binary must not error")
	assert.Empty(t, findings)
}

// Tests for parseDiscoverFindings legacy fallback — was 75% covered

func TestParseVerdict_DiscoverMode_LegacyFindingFormat(t *testing.T) {
	t.Parallel()

	// Legacy format: FINDING: [severity] description (no category or scope_ref)
	output := `VERDICT: REVISE
FINDING: [major] Missing error handling for edge case
FINDING: [minor] Could improve naming`

	result := ParseVerdict("SPEC-X-001", output, "claude", 0, nil)

	assert.Equal(t, VerdictRevise, result.Verdict)
	require.Len(t, result.Findings, 2)
	assert.Equal(t, "major", result.Findings[0].Severity)
	assert.Equal(t, "minor", result.Findings[1].Severity)
	// REQ-07: discover-mode findings leave IDs empty; global sequential IDs
	// are assigned later by DeduplicateFindings in the merge pipeline so that
	// providers do not collide on F-001/F-002.
	assert.Equal(t, "", result.Findings[0].ID)
	assert.Equal(t, "", result.Findings[1].ID)
}

func TestParseVerdict_DiscoverMode_StructuredFindingFormat(t *testing.T) {
	t.Parallel()

	// Structured format: FINDING: [severity] [category] [scope_ref] description
	output := `VERDICT: REVISE
FINDING: [major] [correctness] REQ-001 Missing nil check in auth handler
FINDING: [minor] [style] types.go:42 Inconsistent naming`

	result := ParseVerdict("SPEC-AUTH-001", output, "gemini", 0, nil)

	require.Len(t, result.Findings, 2)
	assert.Equal(t, FindingCategoryCorrectness, result.Findings[0].Category)
	assert.Equal(t, "REQ-001", result.Findings[0].ScopeRef)
	assert.Equal(t, FindingCategoryStyle, result.Findings[1].Category)
	assert.Equal(t, "types.go:42", result.Findings[1].ScopeRef)
}
