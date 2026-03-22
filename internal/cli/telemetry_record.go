package cli

import (
	"fmt"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

// recordParams holds the parsed flags for `auto telemetry record`.
type recordParams struct {
	specID      string
	agent       string
	phase       string
	action      string
	status      string
	files       int
	tokens      int
	qualityMode string
}

// runTelemetryRecord dispatches a telemetry record action (start|agent|end).
// It is extracted from the command RunE for testability.
func runTelemetryRecord(baseDir string, p recordParams) error {
	switch p.action {
	case "start":
		return recordStart(baseDir, p)
	case "agent":
		return recordAgent(baseDir, p)
	case "end":
		return recordEnd(baseDir, p)
	default:
		return fmt.Errorf("telemetry record: unknown action %q (want: start|agent|end)", p.action)
	}
}

// recordStart opens a Recorder, starts the pipeline, and immediately finalizes
// to persist the pipeline_start event. The recorder is intentionally short-lived
// because each agent invocation is a separate process.
func recordStart(baseDir string, p recordParams) error {
	if p.specID == "" {
		return fmt.Errorf("telemetry record start: --spec-id is required")
	}

	rec, err := telemetry.NewRecorder(baseDir, p.specID)
	if err != nil {
		return fmt.Errorf("telemetry record start: %w", err)
	}

	rec.StartPipeline(p.specID, p.qualityMode)
	if p.phase != "" {
		rec.StartPhase(p.phase)
	}
	// Flush: finalize without ending the pipeline (status empty signals in-progress).
	_ = rec.Finalize("")
	return nil
}

// recordAgent appends an agent_run event to an existing pipeline recording.
func recordAgent(baseDir string, p recordParams) error {
	if p.specID == "" {
		return fmt.Errorf("telemetry record agent: --spec-id is required")
	}
	if p.agent == "" {
		return fmt.Errorf("telemetry record agent: --agent is required")
	}

	rec, err := telemetry.NewRecorder(baseDir, p.specID)
	if err != nil {
		return fmt.Errorf("telemetry record agent: %w", err)
	}

	now := time.Now()
	run := telemetry.AgentRun{
		AgentName:       p.agent,
		StartTime:       now,
		EndTime:         now,
		Status:          p.status,
		FilesModified:   p.files,
		EstimatedTokens: p.tokens,
	}

	if p.phase != "" {
		rec.StartPhase(p.phase)
	}
	rec.RecordAgent(run)
	if p.phase != "" {
		rec.EndPhase(p.status)
	}
	_ = rec.Finalize("")
	return nil
}

// recordEnd finalizes a pipeline run with the given status.
func recordEnd(baseDir string, p recordParams) error {
	if p.specID == "" {
		return fmt.Errorf("telemetry record end: --spec-id is required")
	}

	rec, err := telemetry.NewRecorder(baseDir, p.specID)
	if err != nil {
		return fmt.Errorf("telemetry record end: %w", err)
	}

	if p.phase != "" {
		rec.EndPhase(p.status)
	}
	_ = rec.Finalize(p.status)
	return nil
}
