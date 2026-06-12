package spec

// SPEC-SPECREV-002 REQ-002: AppendSelfVerifyEntry must reject an N/A entry
// whose reason is empty or whitespace-only, before any log file is written.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// S3: an N/A entry with an empty reason is rejected and no log is written.
func TestAppendSelfVerifyEntry_RejectsNAWithEmptyReason(t *testing.T) {
	t.Parallel()

	specDir := t.TempDir()
	entry := SelfVerifyEntry{
		Dimension: "security",
		Status:    ChecklistStatusNA,
		Reason:    "",
	}

	err := AppendSelfVerifyEntry(specDir, entry)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason", "error must name the missing reason")

	// The log must not be created when the entry is rejected pre-write.
	_, statErr := os.Stat(filepath.Join(specDir, selfVerifyLogName))
	assert.True(t, os.IsNotExist(statErr), ".self-verify.log must not be written")
}

// S3 variant: whitespace-only reason is treated as empty.
func TestAppendSelfVerifyEntry_RejectsNAWithWhitespaceReason(t *testing.T) {
	t.Parallel()

	specDir := t.TempDir()
	err := AppendSelfVerifyEntry(specDir, SelfVerifyEntry{
		Dimension: "security",
		Status:    ChecklistStatusNA,
		Reason:    "   \t  ",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason")

	_, statErr := os.Stat(filepath.Join(specDir, selfVerifyLogName))
	assert.True(t, os.IsNotExist(statErr))
}

// S4: an N/A entry with a non-empty reason is accepted and persisted.
func TestAppendSelfVerifyEntry_AcceptsNAWithReason(t *testing.T) {
	t.Parallel()

	specDir := t.TempDir()
	err := AppendSelfVerifyEntry(specDir, SelfVerifyEntry{
		Dimension: "security",
		Status:    ChecklistStatusNA,
		Reason:    "doc-only SPEC, no trust boundary",
	})
	require.NoError(t, err)

	data, readErr := os.ReadFile(filepath.Join(specDir, selfVerifyLogName))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), `"status":"N/A"`)
}
