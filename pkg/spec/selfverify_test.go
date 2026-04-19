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

	err := AppendSelfVerifyEntry(t.TempDir(), SelfVerifyEntry{
		Dimension: "style",
		Status:    ChecklistStatus("N/A"),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid self-verify status")
}
