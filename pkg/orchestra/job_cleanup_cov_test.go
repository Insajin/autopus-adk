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

// TestCleanupStaleJobs_KeepsForeignJSONDirectory pins the shared-temp-dir
// invariant: the GC scans os.TempDir(), where unrelated tools keep working
// directories full of JSON. A foreign JSON object decodes into a zero Job whose
// zero CreatedAt otherwise reads as infinitely stale, so the GC used to delete
// the whole directory out from under a live process.
func TestCleanupStaleJobs_KeepsForeignJSONDirectory(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	foreign := filepath.Join(baseDir, "release-coordinate-publish.WgKtut")
	require.NoError(t, os.MkdirAll(foreign, 0o700))
	policies := filepath.Join(foreign, "deployment-policies.json")
	require.NoError(t, os.WriteFile(policies, []byte(`{"branch_policies":[]}`), 0o600))
	variables := filepath.Join(foreign, "repository-variables.json")
	require.NoError(t, os.WriteFile(variables, []byte(`[{"name":"UNRELATED"}]`), 0o600))

	removed, err := CleanupStaleJobs(baseDir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, removed, "foreign JSON is not an orchestra job record")
	assert.FileExists(t, policies)
	assert.FileExists(t, variables)
}

// TestCleanupStaleJobs_KeepsRecordWithoutCreatedAt verifies that a job record
// missing CreatedAt is never treated as stale, even when its ID matches.
func TestCleanupStaleJobs_KeepsRecordWithoutCreatedAt(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	subDir := filepath.Join(baseDir, "autopus-orch-undated")
	require.NoError(t, os.MkdirAll(subDir, 0o700))
	require.NoError(t, (&Job{ID: "undated", Dir: subDir}).Save())

	removed, err := CleanupStaleJobs(baseDir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
	assert.DirExists(t, subDir)
}

// TestCleanupStaleJobs_KeepsDirectoryNotClaimedByRecord verifies a nested stale
// record only authorizes removal of the directory it actually lives in.
func TestCleanupStaleJobs_KeepsDirectoryNotClaimedByRecord(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	host := filepath.Join(baseDir, "unrelated-workdir")
	victim := filepath.Join(baseDir, "victim")
	require.NoError(t, os.MkdirAll(host, 0o700))
	require.NoError(t, os.MkdirAll(victim, 0o700))
	stale := &Job{ID: "misplaced", Dir: victim, CreatedAt: time.Now().Add(-2 * time.Hour)}
	require.NoError(t, writeFlatJobManifest(host, stale))

	removed, err := CleanupStaleJobs(baseDir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
	assert.DirExists(t, host)
	assert.DirExists(t, victim)
}

// TestCleanupStaleJobs_KeepsFlatTargetOutsideBaseDir verifies a flat manifest
// cannot direct the GC at a path outside the scanned root; the manifest itself
// is still reclaimed.
func TestCleanupStaleJobs_KeepsFlatTargetOutsideBaseDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	outside := filepath.Join(root, "outside")
	require.NoError(t, os.MkdirAll(baseDir, 0o700))
	require.NoError(t, os.MkdirAll(outside, 0o700))
	stale := &Job{ID: "escaping", Dir: outside, CreatedAt: time.Now().Add(-2 * time.Hour)}
	require.NoError(t, writeFlatJobManifest(baseDir, stale))

	removed, err := CleanupStaleJobs(baseDir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.DirExists(t, outside, "removal must not escape the scanned root")
	assert.NoFileExists(t, filepath.Join(baseDir, "escaping.json"))
}

// TestCleanupStaleJobs_NeverRemovesBaseDir verifies a stale flat record whose
// Dir is the scanned root reclaims only its own manifest.
func TestCleanupStaleJobs_NeverRemovesBaseDir(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	stale := &Job{ID: "rooted", Dir: baseDir, CreatedAt: time.Now().Add(-2 * time.Hour)}
	require.NoError(t, stale.Save())
	keeper := filepath.Join(baseDir, "keeper.txt")
	require.NoError(t, os.WriteFile(keeper, []byte("keep"), 0o600))

	removed, err := CleanupStaleJobs(baseDir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.DirExists(t, baseDir)
	assert.FileExists(t, keeper)
	assert.NoFileExists(t, filepath.Join(baseDir, "rooted.json"))
}
