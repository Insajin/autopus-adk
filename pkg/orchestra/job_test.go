package orchestra

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJob_SaveAndLoad verifies that a Job can be persisted to disk and loaded
// back with all fields intact.
func TestJob_SaveAndLoad(t *testing.T) {
	t.Parallel()

	// Given: a Job with known fields
	dir := t.TempDir()
	job := &Job{
		ID:        "test-abc123",
		Strategy:  StrategyConsensus,
		Providers: []string{"claude", "codex", "gemini"},
		Prompt:    "refactor auth module",
		CreatedAt: time.Now().Truncate(time.Second),
		TimeoutAt: time.Now().Add(2 * time.Minute).Truncate(time.Second),
		Status:    JobStatusRunning,
		Dir:       dir,
	}

	// When: the job is saved and loaded back
	err := job.Save()
	require.NoError(t, err)

	loaded, err := LoadJob(dir, job.ID)
	require.NoError(t, err)

	// Then: all fields must match
	assert.Equal(t, job.ID, loaded.ID)
	assert.Equal(t, job.Strategy, loaded.Strategy)
	assert.Equal(t, job.Providers, loaded.Providers)
	assert.Equal(t, job.Prompt, loaded.Prompt)
	assert.Equal(t, job.CreatedAt, loaded.CreatedAt)
	assert.Equal(t, job.TimeoutAt, loaded.TimeoutAt)
	assert.Equal(t, job.Status, loaded.Status)
}

// TestJob_CheckStatus tests status resolution based on provider completion state.
func TestJob_CheckStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		job      Job
		expected JobStatus
	}{
		{
			name: "all providers running returns running",
			job: Job{
				Providers: []string{"claude", "codex", "gemini"},
				Results:   map[string]*ProviderResponse{},
				TimeoutAt: time.Now().Add(10 * time.Minute),
			},
			expected: JobStatusRunning,
		},
		{
			name: "some providers done returns partial",
			job: Job{
				Providers: []string{"claude", "codex", "gemini"},
				Results: map[string]*ProviderResponse{
					"claude": {Provider: "claude", Output: "ok"},
				},
				TimeoutAt: time.Now().Add(10 * time.Minute),
			},
			expected: JobStatusPartial,
		},
		{
			name: "all providers done returns done",
			job: Job{
				Providers: []string{"claude", "codex"},
				Results: map[string]*ProviderResponse{
					"claude": {Provider: "claude", Output: "ok"},
					"codex":  {Provider: "codex", Output: "ok"},
				},
				TimeoutAt: time.Now().Add(10 * time.Minute),
			},
			expected: JobStatusDone,
		},
		{
			name: "past timeout returns timeout",
			job: Job{
				Providers: []string{"claude"},
				Results:   map[string]*ProviderResponse{},
				TimeoutAt: time.Now().Add(-1 * time.Minute),
			},
			expected: JobStatusTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.job.CheckStatus()
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestJob_CollectResults verifies that when all providers are done, the merged
// output matches the configured strategy.
func TestJob_CollectResults(t *testing.T) {
	t.Parallel()

	// Given: a completed job with results from all providers
	job := &Job{
		ID:        "collect-001",
		Strategy:  StrategyConsensus,
		Providers: []string{"claude", "codex"},
		Results: map[string]*ProviderResponse{
			"claude": {Provider: "claude", Output: "refactored auth"},
			"codex":  {Provider: "codex", Output: "refactored auth"},
		},
	}

	// When: results are collected
	result, err := job.CollectResults()

	// Then: merged output should reflect the strategy
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Merged)
	assert.Equal(t, StrategyConsensus, result.Strategy)
	assert.Len(t, result.Responses, 2)
}

// TestJob_Cleanup verifies that the job directory and associated panes are
// removed after cleanup.
func TestJob_Cleanup(t *testing.T) {
	t.Parallel()

	// Given: a job with a temp directory
	dir := t.TempDir()
	jobDir := filepath.Join(dir, "job-cleanup-001")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))

	job := &Job{
		ID:  "cleanup-001",
		Dir: jobDir,
	}

	// When: cleanup is called
	err := job.Cleanup()

	// Then: the job directory should be removed
	require.NoError(t, err)
	_, statErr := os.Stat(jobDir)
	assert.True(t, os.IsNotExist(statErr), "job directory should be removed after cleanup")
}

// TestJob_CleanupStaleJobs verifies that jobs older than 1 hour TTL are
// automatically cleaned up via opportunistic GC.
func TestJob_CleanupStaleJobs(t *testing.T) {
	t.Parallel()

	// Given: a jobs directory with one fresh and one stale job
	baseDir := t.TempDir()

	freshDir := filepath.Join(baseDir, "job-fresh")
	staleDir := filepath.Join(baseDir, "job-stale")
	require.NoError(t, os.MkdirAll(freshDir, 0o755))
	require.NoError(t, os.MkdirAll(staleDir, 0o755))

	freshJob := &Job{
		ID:        "fresh",
		Dir:       freshDir,
		CreatedAt: time.Now(),
	}
	staleJob := &Job{
		ID:        "stale",
		Dir:       staleDir,
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}

	// Save both jobs so CleanupStaleJobs can discover them
	require.NoError(t, freshJob.Save())
	require.NoError(t, staleJob.Save())

	// When: stale job cleanup runs
	removed, err := CleanupStaleJobs(baseDir, 1*time.Hour)

	// Then: only the stale job should be removed
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.DirExists(t, freshDir, "fresh job should remain")
	_, statErr := os.Stat(staleDir)
	assert.True(t, os.IsNotExist(statErr), "stale job should be removed")
}

// TestLoadJob_NotFound verifies that LoadJob returns a "not found" error
// for a non-existent job ID.
func TestLoadJob_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := LoadJob(dir, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestLoadJob_InvalidJSON verifies that LoadJob returns an error for malformed JSON.
func TestLoadJob_InvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{invalid"), 0o644))
	_, err := LoadJob(dir, "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

// TestCleanupStaleJobs_NestedSubdir verifies cleanup of stale jobs stored in
// subdirectories (the nested layout used by RunPaneOrchestraDetached).
func TestCleanupStaleJobs_NestedSubdir(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	subDir := filepath.Join(baseDir, "sub-orch")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	staleJob := &Job{
		ID:        "nested-stale",
		Dir:       subDir,
		CreatedAt: time.Now().Add(-3 * time.Hour),
	}
	require.NoError(t, staleJob.Save())

	removed, err := CleanupStaleJobs(baseDir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	_, statErr := os.Stat(subDir)
	assert.True(t, os.IsNotExist(statErr), "nested stale job dir should be removed")
}

// TestCleanupStaleJobs_EmptyDir verifies no errors when base directory has no jobs.
func TestCleanupStaleJobs_EmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	removed, err := CleanupStaleJobs(dir, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}

// TestCleanupStaleJobs_NonexistentDir verifies error on non-existent directory.
func TestCleanupStaleJobs_NonexistentDir(t *testing.T) {
	t.Parallel()

	_, err := CleanupStaleJobs("/nonexistent/path", 1*time.Hour)
	require.Error(t, err)
}

// TestCollectResults_PartialProviders verifies CollectResults when some
// providers have nil results (incomplete job).
func TestCollectResults_PartialProviders(t *testing.T) {
	t.Parallel()

	job := &Job{
		ID:        "partial-001",
		Strategy:  StrategyFastest,
		Providers: []string{"claude", "codex"},
		Results: map[string]*ProviderResponse{
			"claude": {Provider: "claude", Output: "fast result"},
		},
	}

	result, err := job.CollectResults()
	require.NoError(t, err)
	assert.Len(t, result.Responses, 1, "only completed providers should be in responses")
	assert.Equal(t, StrategyFastest, result.Strategy)
}
