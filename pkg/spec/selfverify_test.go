package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendSelfVerifyEntry_AppendsAndRetainsLatest100(t *testing.T) {
	t.Parallel()

	specDir := t.TempDir()

	for i := 0; i < 101; i++ {
		err := AppendSelfVerifyEntry(specDir, SelfVerifyEntry{
			Timestamp: time.Unix(int64(i), 0).UTC(),
			Dimension: "completeness",
			Status:    ChecklistStatusFail,
			Reason:    fmt.Sprintf("issue-%03d", i),
		})
		require.NoError(t, err)
	}

	path := filepath.Join(specDir, selfVerifyLogName)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 100)

	var first SelfVerifyEntry
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &first))
	assert.Equal(t, "issue-001", first.Reason)

	var last SelfVerifyEntry
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &last))
	assert.Equal(t, "issue-100", last.Reason)
}

func TestAppendSelfVerifyEntry_RejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	// N/A is now a valid status (SPEC-SPECREV-001 follow-up); use an
	// unambiguously invalid token to exercise the rejection path.
	err := AppendSelfVerifyEntry(t.TempDir(), SelfVerifyEntry{
		Dimension: "style",
		Status:    ChecklistStatus("MAYBE"),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid self-verify status")
}

// TestAppendSelfVerifyEntry_AcceptsNAStatus pins the SPEC-SPECREV-001 follow-up:
// self-verify entries may now mark a dimension as N/A when the dimension does
// not apply to the SPEC under review (e.g. Q-SEC-* on a doc-only SPEC).
func TestAppendSelfVerifyEntry_AcceptsNAStatus(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := AppendSelfVerifyEntry(dir, SelfVerifyEntry{
		Dimension: "security",
		Status:    ChecklistStatusNA,
		Reason:    "doc-only SPEC, no trust boundary",
	})
	require.NoError(t, err)
}
