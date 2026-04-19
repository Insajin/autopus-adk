package pipeline

import (
	"errors"
	"testing"

	"github.com/insajin/autopus-adk/pkg/learn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStore creates a learn.Store backed by a temp directory.
func newTestStore(t *testing.T) *learn.Store {
	t.Helper()
	store, err := learn.NewStore(t.TempDir())
	require.NoError(t, err)
	return store
}

// TestLearnHookGateFail_NilStore verifies that a nil store is a no-op.
func TestLearnHookGateFail_NilStore(t *testing.T) {
	t.Parallel()
	// Should not panic.
	learnHookGateFail(nil, PhaseValidate, GateValidation, "FAIL output", 0)
}

// TestLearnHookGateFail_Records verifies gate fail entries are written.
func TestLearnHookGateFail_Records(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)

	learnHookGateFail(store, PhaseValidate, GateValidation, "FAIL output", 0)

	entries, err := store.Read()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, learn.EntryTypeGateFail, entries[0].Type)
	assert.Equal(t, string(PhaseValidate), entries[0].Phase)
}

// TestLearnHookGateFail_SeverityEscalation verifies that attempt >= defaultMaxRetries
// records a critical severity entry.
func TestLearnHookGateFail_SeverityEscalation(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)

	learnHookGateFail(store, PhaseValidate, GateValidation, "FAIL", defaultMaxRetries)

	entries, err := store.Read()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, learn.SeverityCritical, entries[0].Severity)
}

// TestLearnHookCoverageGap_NilStore verifies that a nil store is a no-op.
func TestLearnHookCoverageGap_NilStore(t *testing.T) {
	t.Parallel()
	learnHookCoverageGap(nil, "coverage: 62.5% of statements", 85.0)
}

// TestLearnHookCoverageGap table-driven tests.
func TestLearnHookCoverageGap(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		output    string
		threshold float64
		wantEntry bool
	}{
		{
			name:      "below threshold records entry",
			output:    "coverage: 62.5% of statements",
			threshold: 85.0,
			wantEntry: true,
		},
		{
			name:      "at threshold does not record",
			output:    "coverage: 85.0% of statements",
			threshold: 85.0,
			wantEntry: false,
		},
		{
			name:      "above threshold does not record",
			output:    "coverage: 90.0% of statements",
			threshold: 85.0,
			wantEntry: false,
		},
		{
			name:      "no coverage pattern does not record",
			output:    "some random output",
			threshold: 85.0,
			wantEntry: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := newTestStore(t)
			learnHookCoverageGap(store, tc.output, tc.threshold)
			entries, err := store.Read()
			require.NoError(t, err)
			if tc.wantEntry {
				require.Len(t, entries, 1)
				assert.Equal(t, learn.EntryTypeCoverageGap, entries[0].Type)
			} else {
				assert.Empty(t, entries)
			}
		})
	}
}

// TestLearnHookReviewIssue_NilStore verifies that a nil store is a no-op.
func TestLearnHookReviewIssue_NilStore(t *testing.T) {
	t.Parallel()
	learnHookReviewIssue(nil, "REQUEST_CHANGES\nFINDING: [HIGH] missing tests", "SPEC-001")
}

// TestLearnHookReviewIssue table-driven tests.
func TestLearnHookReviewIssue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		output    string
		specID    string
		wantCount int
		wantSev   learn.Severity
		wantPat   string
	}{
		{
			name:      "single finding parsed",
			output:    "REQUEST_CHANGES\nFINDING: [HIGH] missing error handling",
			specID:    "SPEC-001",
			wantCount: 1,
			wantSev:   learn.SeverityHigh,
			wantPat:   "missing error handling",
		},
		{
			name:      "multiple findings parsed individually",
			output:    "FINDING: [CRITICAL] security issue\nFINDING: [LOW] style nit",
			specID:    "SPEC-002",
			wantCount: 2,
		},
		{
			name:      "no findings falls back to full output",
			output:    "REQUEST_CHANGES general feedback",
			specID:    "SPEC-003",
			wantCount: 1,
			wantSev:   learn.SeverityMedium,
			wantPat:   "REQUEST_CHANGES general feedback",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := newTestStore(t)
			learnHookReviewIssue(store, tc.output, tc.specID)
			entries, err := store.Read()
			require.NoError(t, err)
			assert.Len(t, entries, tc.wantCount)
			if tc.wantCount == 1 && tc.wantPat != "" {
				assert.Equal(t, tc.wantPat, entries[0].Pattern)
			}
			if tc.wantCount == 1 && tc.wantSev != "" {
				assert.Equal(t, tc.wantSev, entries[0].Severity)
			}
		})
	}
}

// TestLearnHookExecutorError_NilStore verifies that a nil store is a no-op.
func TestLearnHookExecutorError_NilStore(t *testing.T) {
	t.Parallel()
	learnHookExecutorError(nil, PhaseImplement, errors.New("backend down"))
}

// TestLearnHookExecutorError_Records verifies executor error entries are written.
func TestLearnHookExecutorError_Records(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)

	learnHookExecutorError(store, PhaseImplement, errors.New("timeout"))

	entries, err := store.Read()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, learn.EntryTypeExecutorError, entries[0].Type)
	assert.Equal(t, string(PhaseImplement), entries[0].Phase)
	assert.Equal(t, learn.SeverityHigh, entries[0].Severity)
	assert.Equal(t, "timeout", entries[0].Pattern)
}

// TestParseCoverage table-driven tests.
func TestParseCoverage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  float64
	}{
		{"standard pattern", "coverage: 62.5% of statements", 62.5},
		{"100 percent", "coverage: 100.0% of statements", 100.0},
		{"integer value", "coverage: 75% of statements", 75.0},
		{"no pattern", "some output without coverage", -1},
		{"empty string", "", -1},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseCoverage(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestMapFindingSeverity table-driven tests.
func TestMapFindingSeverity(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  learn.Severity
	}{
		{"CRITICAL", learn.SeverityCritical},
		{"critical", learn.SeverityCritical},
		{"HIGH", learn.SeverityHigh},
		{"high", learn.SeverityHigh},
		{"MEDIUM", learn.SeverityMedium},
		{"medium", learn.SeverityMedium},
		{"LOW", learn.SeverityLow},
		{"low", learn.SeverityLow},
		{"unknown", learn.SeverityLow},
		{"", learn.SeverityLow},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, mapFindingSeverity(tc.input))
		})
	}
}
