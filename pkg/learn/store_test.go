package learn

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore_CreatesDirectory(t *testing.T) {
	t.Parallel()

	// Given: a temp directory without .autopus/learnings/
	dir := t.TempDir()

	// When: creating a new store
	store, err := NewStore(dir)

	// Then: directory is created and store is usable
	require.NoError(t, err)
	assert.NotNil(t, store)

	learningsDir := filepath.Join(dir, ".autopus", "learnings")
	info, err := os.Stat(learningsDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestNewStore_ExistingDirectory(t *testing.T) {
	t.Parallel()

	// Given: directory with existing .autopus/learnings/
	dir := t.TempDir()
	learningsDir := filepath.Join(dir, ".autopus", "learnings")
	require.NoError(t, os.MkdirAll(learningsDir, 0o755))

	// When: creating a new store
	store, err := NewStore(dir)

	// Then: no error, store is usable
	require.NoError(t, err)
	assert.NotNil(t, store)
}

func TestStore_Append_WritesJSONL(t *testing.T) {
	t.Parallel()

	// Given: empty store
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	entry := LearningEntry{
		ID:        "L-001",
		Timestamp: time.Now(),
		Type:      EntryTypeGateFail,
		Phase:     "test",
		Files:     []string{"main.go"},
		Packages:  []string{"main"},
		Pattern:   "test failure",
		Severity:  SeverityMedium,
	}

	// When: appending an entry
	err = store.Append(entry)

	// Then: no error and file contains the entry
	require.NoError(t, err)

	entries, err := store.Read()
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "L-001", entries[0].ID)
}

func TestStore_Append_MultipleEntries(t *testing.T) {
	t.Parallel()

	// Given: empty store
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	// When: appending multiple entries
	for i := 1; i <= 3; i++ {
		entry := LearningEntry{
			ID:       "L-00" + string(rune('0'+i)),
			Type:     EntryTypeGateFail,
			Phase:    "test",
			Severity: SeverityLow,
		}
		require.NoError(t, store.Append(entry))
	}

	// Then: all entries are readable
	entries, err := store.Read()
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

func TestStore_Read_EmptyFile(t *testing.T) {
	t.Parallel()

	// Given: store with no entries
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	// When: reading
	entries, err := store.Read()

	// Then: empty slice, no error
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestStore_Read_InvalidJSON(t *testing.T) {
	t.Parallel()

	// Given: store file with invalid JSON
	dir := t.TempDir()
	learningsDir := filepath.Join(dir, ".autopus", "learnings")
	require.NoError(t, os.MkdirAll(learningsDir, 0o755))
	jsonlPath := filepath.Join(learningsDir, "pipeline.jsonl")
	require.NoError(t, os.WriteFile(jsonlPath, []byte("not valid json\n"), 0o644))

	store, err := NewStore(dir)
	require.NoError(t, err)

	// When: reading
	_, err = store.Read()

	// Then: error on invalid JSON
	assert.Error(t, err)
}

func TestStore_NextID_EmptyStore(t *testing.T) {
	t.Parallel()

	// Given: empty store
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	// When: requesting next ID
	id, err := store.NextID()

	// Then: returns L-001
	require.NoError(t, err)
	assert.Equal(t, "L-001", id)
}

func TestStore_NextID_WithExistingEntries(t *testing.T) {
	t.Parallel()

	// Given: store with 3 entries
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		entry := LearningEntry{
			ID:   "L-00" + string(rune('0'+i)),
			Type: EntryTypeGateFail,
		}
		require.NoError(t, store.Append(entry))
	}

	// When: requesting next ID
	id, err := store.NextID()

	// Then: returns L-004
	require.NoError(t, err)
	assert.Equal(t, "L-004", id)
}

func TestStore_UpdateReuseCount(t *testing.T) {
	t.Parallel()

	// Given: store with one entry
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	entry := LearningEntry{
		ID:         "L-001",
		Type:       EntryTypeFixPattern,
		ReuseCount: 0,
	}
	require.NoError(t, store.Append(entry))

	// When: updating reuse count
	err = store.UpdateReuseCount("L-001")
	require.NoError(t, err)

	// Then: reuse_count is incremented
	entries, err := store.Read()
	require.NoError(t, err)
	assert.Equal(t, 1, entries[0].ReuseCount)
}

func TestStore_UpdateReuseCount_NotFound(t *testing.T) {
	t.Parallel()

	// Given: empty store
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	// When: updating non-existent entry
	err = store.UpdateReuseCount("L-999")

	// Then: error
	assert.Error(t, err)
}
