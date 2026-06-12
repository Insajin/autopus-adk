package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Issue #58 regression locks: PersistFindings must always serialize a valid
// JSON array, never the literal "null" produced by a nil slice.

// downstream tooling even though review.md still carries the findings.
func TestPersistFindings_NilSlice_WritesEmptyArray(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var findings []ReviewFinding // nil slice
	require.NoError(t, PersistFindings(dir, findings))

	path := filepath.Join(dir, "review-findings.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	// Oracle: content is exactly the empty array, not `null`.
	assert.Equal(t, "[]", string(data),
		"nil findings must serialize as [] (issue #58), got %q", string(data))

	// And it must round-trip back to an empty (non-nil-distinguishable) slice.
	var parsed []ReviewFinding
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Empty(t, parsed)
}

// Issue #58: an explicitly empty (non-nil) slice must also serialize as `[]`.
func TestPersistFindings_EmptySlice_WritesEmptyArray(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, PersistFindings(dir, []ReviewFinding{}))

	data, err := os.ReadFile(filepath.Join(dir, "review-findings.json"))
	require.NoError(t, err)
	assert.Equal(t, "[]", string(data))
}
