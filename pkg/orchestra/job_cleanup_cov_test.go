package orchestra

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFlatJobManifest writes a job's JSON manifest directly into baseDir as
// {ID}.json while preserving the job's own Dir field (which points elsewhere).
func writeFlatJobManifest(baseDir string, j *Job) error {
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(baseDir, j.ID+".json"), data, 0o600)
}

// TestCleanupStaleJobs_FlatJSON covers the branch where job JSON files live
// directly in baseDir (not nested in a subdirectory). The stale flat job is
// removed while the fresh one remains.
func TestCleanupStaleJobs_FlatJSON(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()

	// Flat job JSON files live directly in baseDir; each job's working Dir is a
	// separate per-job subdirectory so removing a stale job does not wipe baseDir.
	freshWork := filepath.Join(baseDir, "fresh-work")
	staleWork := filepath.Join(baseDir, "stale-work")
	require.NoError(t, os.MkdirAll(freshWork, 0o755))
	require.NoError(t, os.MkdirAll(staleWork, 0o755))

	// Write the JSON manifests flat in baseDir; the Dir field points at the
	// per-job work dir, so removing a stale job does not wipe baseDir itself.
	freshJob := &Job{ID: "fresh", Dir: freshWork, CreatedAt: time.Now()}
	staleJob := &Job{ID: "stale", Dir: staleWork, CreatedAt: time.Now().Add(-2 * time.Hour)}
	require.NoError(t, writeFlatJobManifest(baseDir, freshJob))
	require.NoError(t, writeFlatJobManifest(baseDir, staleJob))

	removed, err := CleanupStaleJobs(baseDir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, removed, "exactly one stale flat job should be removed")

	assert.FileExists(t, filepath.Join(baseDir, "fresh.json"))
	_, statErr := os.Stat(filepath.Join(baseDir, "stale.json"))
	assert.True(t, os.IsNotExist(statErr), "stale flat job JSON should be gone")
	_, workErr := os.Stat(staleWork)
	assert.True(t, os.IsNotExist(workErr), "stale job work dir should be removed")
}

// TestCleanupStaleJobs_IgnoresNonJSON verifies non-JSON files and unparsable
// JSON are skipped without error and counted as not removed.
func TestCleanupStaleJobs_IgnoresNonJSON(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "notes.txt"), []byte("hi"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "broken.json"), []byte("{not-json"), 0o600))

	removed, err := CleanupStaleJobs(baseDir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, removed, "non-JSON and unparsable files are skipped")
	assert.FileExists(t, filepath.Join(baseDir, "notes.txt"))
}

// TestCleanupStaleJobs_MissingBaseDir covers the os.ReadDir error branch.
func TestCleanupStaleJobs_MissingBaseDir(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	removed, err := CleanupStaleJobs(missing, 1*time.Hour)
	require.Error(t, err)
	assert.Equal(t, 0, removed)
	assert.Contains(t, err.Error(), "read dir")
}

// TestCleanupStaleJobs_FreshFlatRetained verifies a fresh flat job is retained
// and the count is zero when nothing is stale.
func TestCleanupStaleJobs_FreshFlatRetained(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	job := &Job{ID: "recent", Dir: baseDir, CreatedAt: time.Now()}
	require.NoError(t, job.Save())

	removed, err := CleanupStaleJobs(baseDir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
	assert.FileExists(t, filepath.Join(baseDir, "recent.json"))
}
