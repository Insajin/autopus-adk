// Package telemetry provides JSONL event recording for pipeline execution.
package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Recorder writes telemetry events to a JSONL file for a single pipeline run.
// It is safe for concurrent use via an internal mutex.
type Recorder struct {
	mu           sync.Mutex
	file         *os.File
	baseDir      string
	specID       string
	pipelineStart time.Time
	qualityMode  string
	phases       []PhaseRecord
	currentPhase *PhaseRecord
}

// NewRecorder creates a Recorder that appends events to
// .autopus/telemetry/{date}-{specID}.jsonl under baseDir.
// The specID is sanitized with filepath.Base to prevent path traversal (R15).
// @AX:ANCHOR: [AUTO] public API boundary — callers depend on directory layout .autopus/telemetry/
// @AX:REASON: pipeline orchestrator, CLI init, and test harness all use this constructor
// @AX:NOTE: [AUTO] filepath.Base(filepath.Clean(specID)) — path traversal prevention per R15
func NewRecorder(baseDir, specID string) (*Recorder, error) {
	// Sanitize: strip any directory components to prevent path traversal.
	safeID := filepath.Base(filepath.Clean(specID))
	if safeID == "." || safeID == "/" {
		safeID = "unknown"
	}

	telDir := filepath.Join(baseDir, ".autopus", "telemetry")
	if err := os.MkdirAll(telDir, 0o755); err != nil {
		return nil, fmt.Errorf("telemetry: create dir: %w", err)
	}

	date := time.Now().UTC().Format("2006-01-02")
	filename := fmt.Sprintf("%s-%s.jsonl", date, safeID)
	path := filepath.Join(telDir, filename)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("telemetry: open file: %w", err)
	}

	return &Recorder{
		file:    f,
		baseDir: baseDir,
		specID:  safeID,
	}, nil
}

// StartPipeline records a pipeline_start event and initialises internal state.
func (r *Recorder) StartPipeline(specID, qualityMode string) {
	r.mu.Lock()
	r.pipelineStart = time.Now()
	r.qualityMode = qualityMode
	r.phases = nil
	r.mu.Unlock()

	_ = r.writeEvent(EventTypePipelineStart, map[string]string{
		"spec_id":      specID,
		"quality_mode": qualityMode,
	})
}

// StartPhase records a phase_start event and begins tracking a new phase.
func (r *Recorder) StartPhase(name string) {
	r.mu.Lock()
	r.currentPhase = &PhaseRecord{
		Name:      name,
		StartTime: time.Now(),
	}
	r.mu.Unlock()

	_ = r.writeEvent(EventTypePhaseStart, map[string]string{"name": name})
}

// RecordAgent records an agent_run event and appends it to the current phase.
func (r *Recorder) RecordAgent(run AgentRun) {
	r.mu.Lock()
	if r.currentPhase != nil {
		r.currentPhase.Agents = append(r.currentPhase.Agents, run)
	}
	r.mu.Unlock()

	_ = r.writeEvent(EventTypeAgentRun, run)
}

// EndPhase records a phase_end event and finalises the current phase record.
func (r *Recorder) EndPhase(status string) {
	r.mu.Lock()
	if r.currentPhase != nil {
		r.currentPhase.EndTime = time.Now()
		r.currentPhase.Duration = time.Since(r.currentPhase.StartTime)
		r.currentPhase.Status = status
		r.phases = append(r.phases, *r.currentPhase)
		r.currentPhase = nil
	}
	r.mu.Unlock()

	_ = r.writeEvent(EventTypePhaseEnd, map[string]string{"status": status})
}

// Finalize records a pipeline_end event, closes the file, and returns the
// complete PipelineRun summary.
// @AX:ANCHOR: [AUTO] must be called exactly once per Recorder — subsequent writes are no-ops
// @AX:REASON: writeEvent guards on r.file == nil; double-Finalize is safe but silent
func (r *Recorder) Finalize(finalStatus string) PipelineRun {
	r.mu.Lock()
	end := time.Now()
	run := PipelineRun{
		SpecID:        r.specID,
		StartTime:     r.pipelineStart,
		EndTime:       end,
		TotalDuration: end.Sub(r.pipelineStart),
		Phases:        r.phases,
		FinalStatus:   finalStatus,
		QualityMode:   r.qualityMode,
	}
	r.mu.Unlock()

	_ = r.writeEvent(EventTypePipelineEnd, run)

	r.mu.Lock()
	if r.file != nil {
		_ = r.file.Close()
		r.file = nil
	}
	r.mu.Unlock()

	return run
}

// CleanExpired deletes JSONL files in the telemetry directory that are older
// than retentionDays (R10: data retention policy).
func (r *Recorder) CleanExpired(retentionDays int) error {
	telDir := filepath.Join(r.baseDir, ".autopus", "telemetry")
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	entries, err := os.ReadDir(telDir)
	if err != nil {
		return fmt.Errorf("telemetry: read dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(telDir, entry.Name())
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("telemetry: remove %s: %w", path, err)
			}
		}
	}
	return nil
}

// writeEvent marshals data to JSON, wraps it in an Event, and appends one line
// to the JSONL file. Safe to call concurrently via the internal mutex.
func (r *Recorder) writeEvent(eventType string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("telemetry: marshal data: %w", err)
	}

	event := Event{
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Data:      payload,
	}

	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("telemetry: marshal event: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// @AX:NOTE: [AUTO] r.file == nil after Finalize() — writes become no-ops, not errors
	if r.file == nil {
		return nil
	}
	_, err = fmt.Fprintf(r.file, "%s\n", line)
	return err
}
