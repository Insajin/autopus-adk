package orchestra

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// JobStatus represents the current state of a detached orchestra job.
type JobStatus string

const (
	JobStatusRunning JobStatus = "running"
	JobStatusPartial JobStatus = "partial"
	JobStatusDone    JobStatus = "done"
	JobStatusTimeout JobStatus = "timeout"
	JobStatusError   JobStatus = "error"
)

// Job represents a detached orchestra execution that persists to disk.
// @AX:NOTE [AUTO] public API boundary — Job is the persistence model for detach mode; LoadJob/Save form the serialization contract; fan_in=3 (detach.go, orchestra_job.go, CleanupStaleJobs)
type Job struct {
	ID          string                       `json:"id"`
	RunID       string                       `json:"run_id,omitempty"`
	Strategy    Strategy                     `json:"strategy"`
	Providers   []string                     `json:"providers"`
	Prompt      string                       `json:"prompt"`
	PromptHash  string                       `json:"prompt_hash,omitempty"`
	CreatedAt   time.Time                    `json:"created_at"`
	TimeoutAt   time.Time                    `json:"timeout_at"`
	Status      JobStatus                    `json:"status"`
	Dir         string                       `json:"dir"`
	ArtifactDir string                       `json:"artifact_dir,omitempty"`
	Results     map[string]*ProviderResponse `json:"results,omitempty"`
	PaneIDs     map[string]string            `json:"pane_ids,omitempty"`
	Terminal    string                       `json:"terminal,omitempty"`
	Judge       string                       `json:"judge,omitempty"`
	OutputFiles map[string]string            `json:"output_files,omitempty"`
}

// Save writes the job as JSON to {ID}.json in the job's Dir.
// @AX:NOTE [AUTO] file layout contract — callers expect {Dir}/{ID}.json; changing path format breaks LoadJob and CleanupStaleJobs
func (j *Job) Save() error {
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	path := filepath.Join(j.Dir, j.ID+".json")
	// SEC: owner-only permissions to protect prompt/result data
	return os.WriteFile(path, data, 0o600)
}

// LoadJob reads a job from {id}.json in the given directory.
// SEC: validates that id contains only safe characters to prevent path traversal.
// @AX:ANCHOR [AUTO] fan_in=3 — called by CLI status/wait/result cmds, CleanupStaleJobs, and cleanupJobsInDir
func LoadJob(dir, id string) (*Job, error) {
	if !validProviderName.MatchString(id) || strings.Contains(id, "..") {
		return nil, fmt.Errorf("job %q not found in %s", id, dir)
	}
	path := filepath.Join(dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("job %q not found in %s", id, dir)
		}
		return nil, fmt.Errorf("read job: %w", err)
	}
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}
	return &job, nil
}

// CheckStatus determines the current status based on timeout and result completion.
func (j *Job) CheckStatus() JobStatus {
	if time.Now().After(j.TimeoutAt) {
		return JobStatusTimeout
	}
	completed := 0
	for _, p := range j.Providers {
		if j.Results[p] != nil {
			completed++
		}
	}
	if completed == len(j.Providers) {
		return JobStatusDone
	}
	if completed > 0 {
		return JobStatusPartial
	}
	return JobStatusRunning
}

// CollectResults builds an OrchestraResult from stored provider responses.
func (j *Job) CollectResults() (*OrchestraResult, error) {
	var responses []ProviderResponse
	for _, p := range j.Providers {
		r := j.Results[p]
		if r == nil {
			continue
		}
		responses = append(responses, *r)
	}
	cfg := OrchestraConfig{Strategy: j.Strategy, JudgeProvider: j.Judge}
	merged, summary := mergeByStrategy(j.Strategy, responses, cfg)
	return &OrchestraResult{
		Strategy:  j.Strategy,
		Responses: responses,
		Merged:    merged,
		Summary:   summary,
		RunID:     j.RunID,
		Reliability: &ReliabilitySummary{
			RunID:       j.RunID,
			ArtifactDir: j.ArtifactDir,
		},
	}, nil
}

// Cleanup removes the job directory and all its contents.
func (j *Job) Cleanup() error {
	return os.RemoveAll(j.Dir)
}

// CleanupStaleJobs scans baseDir for job subdirectories containing JSON files
// and removes those whose CreatedAt + ttl is in the past. Returns the count removed.
// @AX:NOTE [AUTO] REQ-11 opportunistic GC — called at start of every orchestra command; scans both flat and nested job dirs
func CleanupStaleJobs(baseDir string, ttl time.Duration) (int, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return 0, fmt.Errorf("read dir: %w", err)
	}
	removed := 0
	cutoff := time.Now().Add(-ttl)
	for _, e := range entries {
		if e.IsDir() {
			// Scan subdirectory for job JSON files
			subDir := filepath.Join(baseDir, e.Name())
			n, _ := cleanupJobsInDir(subDir, cutoff)
			removed += n
			continue
		}
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		job, err := LoadJob(baseDir, id)
		if err != nil {
			continue
		}
		if job.CreatedAt.Before(cutoff) {
			if job.Dir != "" {
				_ = os.RemoveAll(job.Dir)
			}
			_ = os.Remove(filepath.Join(baseDir, e.Name()))
			removed++
		}
	}
	return removed, nil
}

// cleanupJobsInDir removes stale jobs found as JSON files in dir.
func cleanupJobsInDir(dir string, cutoff time.Time) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		job, err := LoadJob(dir, id)
		if err != nil {
			continue
		}
		if job.CreatedAt.Before(cutoff) {
			_ = os.RemoveAll(dir)
			removed++
			break // directory removed, no more files to check
		}
	}
	return removed, nil
}
