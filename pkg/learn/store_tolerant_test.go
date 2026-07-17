package learn

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_ReadTolerant_FallbackTime(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	learningsDir := filepath.Join(dir, ".autopus", "learnings")
	require.NoError(t, os.MkdirAll(learningsDir, 0o755))
	jsonlPath := filepath.Join(learningsDir, "pipeline.jsonl")

	raw := `{"id":"L-001","timestamp":"2026-06-14T14:31:55+09:00","type":"gate_fail","packages":["pkg/learn"]}
{"id":"L-002","timestamp":"2026-06-14T05:31:55Z","type":"gate_fail","packages":["pkg/learn"]}
{"id":"L-003","timestamp":"2026-06-14T14:31:55+0900","type":"gate_fail","packages":["pkg/learn"]}
`
	require.NoError(t, os.WriteFile(jsonlPath, []byte(raw), 0o644))

	store, err := NewStore(dir)
	require.NoError(t, err)

	entries, skips, err := store.ReadTolerant()
	require.NoError(t, err)
	assert.Len(t, entries, 3)
	assert.Len(t, skips, 0)

	assert.Equal(t, "L-003", entries[2].ID)
	pruned, err := Prune(store, 100000)
	require.NoError(t, err)
	assert.Equal(t, 0, pruned)

	content, err := os.ReadFile(jsonlPath)
	require.NoError(t, err)
	contentStr := string(content)
	assert.Contains(t, contentStr, "+09:00")
	// Non-vacuous canonicalization check: the colon-less offset must not
	// survive a rewrite anywhere in the file (not just before a newline).
	assert.NotContains(t, contentStr, "+0900")
}

// TestStore_S3_CanonicalRewritePreservesGarbageAndPrunesAged is the exact
// acceptance.md S3 oracle: a fresh survivor with a colon-less +0900 offset,
// a 60-day-old normal entry that must age out, and an unparseable line that
// must survive rewrite untouched, all rewritten by a single real prune call.
func TestStore_S3_CanonicalRewritePreservesGarbageAndPrunesAged(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	learningsDir := filepath.Join(dir, ".autopus", "learnings")
	require.NoError(t, os.MkdirAll(learningsDir, 0o755))
	jsonlPath := filepath.Join(learningsDir, "pipeline.jsonl")

	survivorTS := time.Now().Format("2006-01-02T15:04:05-0700") // fresh, colon-less offset
	agedOutTS := time.Now().Add(-60 * 24 * time.Hour).Format(time.RFC3339)

	raw := fmt.Sprintf(`{"id":"L-SURVIVOR","timestamp":%q}
{"id":"L-AGED","timestamp":%q}
GARBAGE-LINE-XYZ
`, survivorTS, agedOutTS)
	require.NoError(t, os.WriteFile(jsonlPath, []byte(raw), 0o644))

	store, err := NewStore(dir)
	require.NoError(t, err)

	removed, err := Prune(store, 30)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	content, err := os.ReadFile(jsonlPath)
	require.NoError(t, err)
	contentStr := string(content)

	canonicalOffset := regexp.MustCompile(`T[0-9:.]+(Z|[+-][0-9]{2}:[0-9]{2})"`)
	assert.Regexp(t, canonicalOffset, contentStr)
	assert.NotContains(t, contentStr, "+0900")
	assert.Equal(t, 1, strings.Count(contentStr, "GARBAGE-LINE-XYZ"))
	assert.Contains(t, contentStr, "L-SURVIVOR")
	assert.NotContains(t, contentStr, "L-AGED")

	// Original relative order preserved: the garbage line was line 3 and
	// must remain after the survivor entry in the rewritten file.
	survivorIdx := strings.Index(contentStr, "L-SURVIVOR")
	garbageIdx := strings.Index(contentStr, "GARBAGE-LINE-XYZ")
	require.NotEqual(t, -1, survivorIdx)
	require.NotEqual(t, -1, garbageIdx)
	assert.Less(t, survivorIdx, garbageIdx)
}

func TestStore_ReadTolerant_SkipsGarbage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	learningsDir := filepath.Join(dir, ".autopus", "learnings")
	require.NoError(t, os.MkdirAll(learningsDir, 0o755))
	jsonlPath := filepath.Join(learningsDir, "pipeline.jsonl")

	raw := `{"id":"L-001","timestamp":"2026-06-14T14:31:55Z"}
{"id":"L-002","timestamp":"2026-06-14T15:31:55Z"}
{"id":"L-BAD","timestamp":
`
	require.NoError(t, os.WriteFile(jsonlPath, []byte(raw), 0o644))

	store, err := NewStore(dir)
	require.NoError(t, err)

	entries, skips, err := store.ReadTolerant()
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Len(t, skips, 1)
	assert.Equal(t, 3, skips[0].Line)
	assert.Equal(t, `{"id":"L-BAD","timestamp":`, skips[0].Raw)

	t1 := time.Now().Add(-40 * 24 * time.Hour).Format(time.RFC3339)
	t2 := time.Now().Format(time.RFC3339)
	rawPreserve := fmt.Sprintf(`{"id":"L-001","timestamp":%q}
GARBAGE-LINE-XYZ
{"id":"L-002","timestamp":%q}
`, t1, t2)
	require.NoError(t, os.WriteFile(jsonlPath, []byte(rawPreserve), 0o644))

	removed, err := Prune(store, 30)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	newContent, err := os.ReadFile(jsonlPath)
	require.NoError(t, err)
	newStr := string(newContent)
	assert.Contains(t, newStr, "GARBAGE-LINE-XYZ")
	assert.Contains(t, newStr, "L-002")
	assert.NotContains(t, newStr, "L-001")
}
