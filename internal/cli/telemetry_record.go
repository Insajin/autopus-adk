package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

// recordParams holds the parsed flags for `auto telemetry record`.
type recordParams struct {
	specID           string
	agent            string
	phase            string
	action           string
	status           string
	files            int
	tokens           int
	qualityMode      string
	usageJSON        string
	acceptanceStatus string
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
	usage, err := loadUsageJSON(p.usageJSON)
	if err != nil {
		return fmt.Errorf("telemetry record agent: %w", err)
	}

	rec, err := telemetry.NewRecorder(baseDir, p.specID)
	if err != nil {
		return fmt.Errorf("telemetry record agent: %w", err)
	}

	now := time.Now()
	run := telemetry.AgentRun{
		AgentName:        p.agent,
		SpecID:           p.specID,
		Phase:            p.phase,
		StartTime:        now,
		EndTime:          now,
		Status:           p.status,
		FilesModified:    p.files,
		EstimatedTokens:  p.tokens,
		AcceptanceStatus: p.acceptanceStatus,
		Usage:            usage,
	}
	if len(usage) > 0 {
		first := usage[0]
		run.TaskID = first.TaskID
		run.RunID = first.RunID
		if len(usage) == 1 {
			run.CallID = first.CallID
		}
		run.Attempt = first.Attempt
		run.Provider = first.Provider
		run.Model = first.Model
		run.Effort = first.Effort
		if first.Phase != "" {
			run.Phase = first.Phase
		}
		run.Role = first.Role
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

func loadUsageJSON(path string) ([]telemetry.UsageEnvelope, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read --usage-json: %w", err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("decode --usage-json: empty document")
	}
	var envelopes []telemetry.UsageEnvelope
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if trimmed[0] == '[' {
		if err := decoder.Decode(&envelopes); err != nil {
			return nil, fmt.Errorf("decode --usage-json: %w", err)
		}
	} else {
		var envelope telemetry.UsageEnvelope
		if err := decoder.Decode(&envelope); err != nil {
			return nil, fmt.Errorf("decode --usage-json: %w", err)
		}
		envelopes = []telemetry.UsageEnvelope{envelope}
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	if len(envelopes) == 0 {
		return nil, fmt.Errorf("decode --usage-json: at least one envelope is required")
	}
	for i, envelope := range envelopes {
		if err := telemetry.ValidateUsageEnvelope(envelope); err != nil {
			return nil, fmt.Errorf("validate --usage-json envelope %d: %w", i, err)
		}
	}
	return deduplicateUsageJSON(envelopes)
}

func deduplicateUsageJSON(envelopes []telemetry.UsageEnvelope) ([]telemetry.UsageEnvelope, error) {
	unique := make([]telemetry.UsageEnvelope, 0, len(envelopes))
	for _, candidate := range envelopes {
		duplicate := false
		for _, prior := range unique {
			if prior.RunID != candidate.RunID || prior.CallID != candidate.CallID {
				continue
			}
			aggregate := telemetry.AggregateUsage([]telemetry.UsageEnvelope{prior, candidate})
			if aggregate.PromotionBlocked {
				return nil, fmt.Errorf("validate --usage-json: %s", aggregate.UnavailableReason)
			}
			duplicate = true
			break
		}
		if !duplicate {
			unique = append(unique, candidate)
		}
	}
	return unique, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode --usage-json: multiple JSON values")
		}
		return fmt.Errorf("decode --usage-json: %w", err)
	}
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
