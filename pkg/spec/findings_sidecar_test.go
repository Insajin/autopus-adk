package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadFindings_OldArrayShape is the AC-RINT-COMPAT-8 oracle: a
// review-findings.json written by the prior schema as a top-level array with
// three findings and no coverage fields must still load without error.
func TestLoadFindings_OldArrayShape(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	oldSidecar := `[
  {"ID":"F-001","Provider":"claude","Severity":"major","Category":"correctness","ScopeRef":"REQ-001","Description":"first","Status":"open"},
  {"ID":"F-002","Provider":"codex","Severity":"minor","Category":"style","ScopeRef":"REQ-002","Description":"second","Status":"resolved"},
  {"ID":"F-003","Provider":"gemini","Severity":"suggestion","Category":"completeness","ScopeRef":"REQ-003","Description":"third","Status":"deferred"}
]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "review-findings.json"), []byte(oldSidecar), 0o644))

	findings, err := LoadFindings(dir)
	require.NoError(t, err)
	require.Len(t, findings, 3, "prior-schema array must load all three findings")
	assert.Equal(t, "F-001", findings[0].ID)
	assert.Equal(t, "F-003", findings[2].ID)
}

// TestLoadFindingsWithCoverage_OldArrayShapeHasEmptyCoverage verifies the
// prior-schema array loads with an empty (additive-optional) coverage set.
func TestLoadFindingsWithCoverage_OldArrayShapeHasEmptyCoverage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, PersistFindings(dir, []ReviewFinding{{ID: "F-001", Description: "x", Status: FindingStatusOpen}}))

	findings, coverages, err := LoadFindingsWithCoverage(dir)
	require.NoError(t, err)
	assert.Len(t, findings, 1)
	assert.Empty(t, coverages, "prior-schema array carries no coverage fields")
}

// TestPersistFindingsWithCoverage_WritesObjectShape verifies the new sidecar is
// a JSON object carrying both findings and doc_coverages.
func TestPersistFindingsWithCoverage_WritesObjectShape(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	findings := []ReviewFinding{{ID: "F-001", Description: "issue", Status: FindingStatusOpen}}
	coverages := []DocCoverage{{Name: "research.md", Injected: 200, Total: 250, Percent: 80, Complete: false}}

	require.NoError(t, PersistFindingsWithCoverage(dir, findings, coverages))

	data, err := os.ReadFile(filepath.Join(dir, "review-findings.json"))
	require.NoError(t, err)
	trimmed := strings.TrimLeft(string(data), " \t\r\n")
	require.NotEmpty(t, trimmed)
	assert.Equal(t, byte('{'), trimmed[0], "new sidecar must be a JSON object")
	assert.Contains(t, string(data), "\"findings\"")
	assert.Contains(t, string(data), "\"doc_coverages\"")
}

// TestPersistFindingsWithCoverage_RoundTrips verifies the object shape loads back
// through both LoadFindings and LoadFindingsWithCoverage.
func TestPersistFindingsWithCoverage_RoundTrips(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	findings := []ReviewFinding{
		{ID: "F-001", Description: "a", Status: FindingStatusOpen, Category: FindingCategoryCorrectness},
		{ID: "F-002", Description: "b", Status: FindingStatusResolved, Category: FindingCategoryStyle},
	}
	coverages := []DocCoverage{
		{Name: "plan.md", Injected: 150, Total: 150, Percent: 100, Complete: true},
		{Name: "research.md", Injected: 200, Total: 250, Percent: 80, Complete: false},
	}
	require.NoError(t, PersistFindingsWithCoverage(dir, findings, coverages))

	gotFindings, err := LoadFindings(dir)
	require.NoError(t, err)
	assert.Equal(t, findings, gotFindings, "object-shape findings must round-trip via LoadFindings")

	gotFindings2, gotCoverages, err := LoadFindingsWithCoverage(dir)
	require.NoError(t, err)
	assert.Equal(t, findings, gotFindings2)
	assert.Equal(t, coverages, gotCoverages, "coverage must round-trip via LoadFindingsWithCoverage")
}

// TestPersistFindingsWithCoverage_NilInputsSerializeEmptyArrays verifies nil
// findings and nil coverages serialize as empty arrays, never JSON null.
func TestPersistFindingsWithCoverage_NilInputsSerializeEmptyArrays(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, PersistFindingsWithCoverage(dir, nil, nil))

	data, err := os.ReadFile(filepath.Join(dir, "review-findings.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "null", "nil slices must serialize as [] not null")

	findings, coverages, err := LoadFindingsWithCoverage(dir)
	require.NoError(t, err)
	assert.Empty(t, findings)
	assert.Empty(t, coverages)
}

// TestLoadFindings_MissingFileReturnsEmpty preserves the existing contract that
// a missing sidecar is not an error.
func TestLoadFindings_MissingFileReturnsEmpty(t *testing.T) {
	t.Parallel()

	findings, err := LoadFindings(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, findings)
}
