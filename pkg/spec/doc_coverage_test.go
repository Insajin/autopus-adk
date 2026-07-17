package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComputeCoverage_OracleValues locks the coverage formula
// (INV-001): percent = floor(injected*100/total), complete = injected >= total.
func TestComputeCoverage_OracleValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		injected     int
		total        int
		wantPercent  int
		wantComplete bool
	}{
		{"partial-80", 200, 250, 80, false},
		{"full-equal", 150, 150, 100, true},
		{"full-exact", 250, 250, 100, true},
		{"floor-33", 1, 3, 33, false},
		{"empty-doc", 0, 0, 100, true},
		{"over-injected", 260, 250, 100, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cov := ComputeCoverage("research.md", tc.injected, tc.total)
			assert.Equal(t, "research.md", cov.Name)
			assert.Equal(t, tc.injected, cov.Injected)
			assert.Equal(t, tc.total, cov.Total)
			assert.Equal(t, tc.wantPercent, cov.Percent, "percent must be floor(injected*100/total)")
			assert.Equal(t, tc.wantComplete, cov.Complete, "complete must be injected>=total")
		})
	}
}

// TestAuxDocCoverages_SmallBudgetCompressesLargestDoc is the AC-RINT-COV-1
// oracle: research.md (250) compresses to 200 injected while plan.md (150)
// stays whole under a deliberately small total budget of 350 lines.
func TestAuxDocCoverages_SmallBudgetCompressesLargestDoc(t *testing.T) {
	t.Parallel()

	specDir := t.TempDir()
	writeLinesDoc(t, specDir, "plan.md", 150, "plan line")
	writeLinesDoc(t, specDir, "research.md", 250, "research line")

	coverages := AuxDocCoverages(specDir, 350)

	byName := coverageByName(coverages)
	research := byName["research.md"]
	assert.Equal(t, 200, research.Injected, "research.md injected lines")
	assert.Equal(t, 250, research.Total, "research.md total lines")
	assert.Equal(t, 80, research.Percent, "research.md coverage percent")
	assert.False(t, research.Complete, "research.md must be marked incomplete")

	plan := byName["plan.md"]
	assert.Equal(t, 150, plan.Injected, "plan.md injected lines")
	assert.Equal(t, 150, plan.Total, "plan.md total lines")
	assert.Equal(t, 100, plan.Percent, "plan.md coverage percent")
	assert.True(t, plan.Complete, "plan.md must be marked complete")
}

// TestAuxDocCoverages_DefaultBudgetFullInjection proves that under the generous
// default total budget both documents inject at 100 percent (REQ-RINT-FULL-02).
func TestAuxDocCoverages_DefaultBudgetFullInjection(t *testing.T) {
	t.Parallel()

	specDir := t.TempDir()
	writeLinesDoc(t, specDir, "plan.md", 150, "plan line")
	writeLinesDoc(t, specDir, "research.md", 250, "research line")

	coverages := AuxDocCoverages(specDir, DefaultAuxTotalBudgetLines)

	for _, cov := range coverages {
		assert.Equalf(t, 100, cov.Percent, "%s must be fully injected under default budget", cov.Name)
		assert.Truef(t, cov.Complete, "%s must be complete under default budget", cov.Name)
		assert.Equalf(t, cov.Total, cov.Injected, "%s injected must equal total", cov.Name)
	}
}

// TestAuxDocCoverages_MotivatingSpecSetFitsDefault proves the real motivating
// document set (358+429+404 = 1191 lines) injects in full under the default
// budget, disproving the head-only loss in SPEC-DESKTOP-DEVICE-SETUP-001.
func TestAuxDocCoverages_MotivatingSpecSetFitsDefault(t *testing.T) {
	t.Parallel()

	specDir := t.TempDir()
	writeLinesDoc(t, specDir, "plan.md", 358, "plan line")
	writeLinesDoc(t, specDir, "research.md", 429, "research line")
	writeLinesDoc(t, specDir, "acceptance.md", 404, "acceptance line")

	coverages := AuxDocCoverages(specDir, DefaultAuxTotalBudgetLines)

	require.Len(t, coverages, 3)
	for _, cov := range coverages {
		assert.Truef(t, cov.Complete, "%s must inject fully at the default budget", cov.Name)
		assert.Equalf(t, 100, cov.Percent, "%s coverage percent", cov.Name)
	}
}

// TestAuxDocCoverages_MissingDocsSkipped verifies absent auxiliary files do not
// produce coverage rows.
func TestAuxDocCoverages_MissingDocsSkipped(t *testing.T) {
	t.Parallel()

	specDir := t.TempDir()
	writeLinesDoc(t, specDir, "plan.md", 20, "plan line")

	coverages := AuxDocCoverages(specDir, DefaultAuxTotalBudgetLines)

	require.Len(t, coverages, 1)
	assert.Equal(t, "plan.md", coverages[0].Name)
}

// TestRenderObservationCoverage_ListsRows verifies the review.md section render
// helper emits a header and one row per document with concrete numbers.
func TestRenderObservationCoverage_ListsRows(t *testing.T) {
	t.Parallel()

	rendered := RenderObservationCoverage([]DocCoverage{
		{Name: "plan.md", Injected: 150, Total: 150, Percent: 100, Complete: true},
		{Name: "research.md", Injected: 200, Total: 250, Percent: 80, Complete: false},
	})

	assert.Contains(t, rendered, "## Observation Coverage")
	assert.Contains(t, rendered, "plan.md")
	assert.Contains(t, rendered, "research.md")
	assert.Contains(t, rendered, "80%")
	assert.Contains(t, rendered, "100%")
	assert.Contains(t, rendered, "200")
	assert.Contains(t, rendered, "250")
}

// TestRenderObservationCoverage_EmptyReturnsNoSection verifies no section is
// emitted when there are no coverage rows.
func TestRenderObservationCoverage_EmptyReturnsNoSection(t *testing.T) {
	t.Parallel()

	assert.Empty(t, RenderObservationCoverage(nil))
}

// TestAllDocumentsFullyObserved covers the gate helper: every complete row is
// fully observed, any incomplete row is not, and an empty set is fully observed.
func TestAllDocumentsFullyObserved(t *testing.T) {
	t.Parallel()

	assert.True(t, AllDocumentsFullyObserved(nil), "no aux docs means nothing was truncated")
	assert.True(t, AllDocumentsFullyObserved([]DocCoverage{
		{Name: "plan.md", Complete: true},
		{Name: "research.md", Complete: true},
	}))
	assert.False(t, AllDocumentsFullyObserved([]DocCoverage{
		{Name: "plan.md", Complete: true},
		{Name: "research.md", Complete: false},
	}))
}

// writeLinesDoc writes a document with exactly n lines (no trailing newline) so
// strings.Split(content, "\n") yields exactly n entries.
func writeLinesDoc(t *testing.T, dir, name string, n int, token string) {
	t.Helper()
	lines := make([]string, n)
	for i := range lines {
		lines[i] = token
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(strings.Join(lines, "\n")), 0o644))
}

func coverageByName(coverages []DocCoverage) map[string]DocCoverage {
	byName := make(map[string]DocCoverage, len(coverages))
	for _, cov := range coverages {
		byName[cov.Name] = cov
	}
	return byName
}
