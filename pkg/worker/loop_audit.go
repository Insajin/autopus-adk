package worker

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/audit"
)

// AuditEvent represents a structured audit log entry for task execution.
type AuditEvent struct {
	TaskID     string  `json:"task_id"`
	Event      string  `json:"event"` // "started", "completed", "failed"
	Timestamp  string  `json:"timestamp"`
	DurationMS int64   `json:"duration_ms,omitempty"`
	CostUSD    float64 `json:"cost_usd,omitempty"`
}

// writeAuditEvent writes a JSON Lines entry to the audit writer.
// Returns nil if writer is nil (audit disabled).
func writeAuditEvent(w *audit.RotatingWriter, evt AuditEvent) error {
	if w == nil {
		return nil
	}
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// newAuditStartedEvent creates an audit event for task start.
func newAuditStartedEvent(taskID string) AuditEvent {
	return AuditEvent{
		TaskID:    taskID,
		Event:     "started",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// newAuditCompletedEvent creates an audit event for task completion.
func newAuditCompletedEvent(taskID string, durationMS int64, costUSD float64) AuditEvent {
	return AuditEvent{
		TaskID:     taskID,
		Event:      "completed",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		DurationMS: durationMS,
		CostUSD:    costUSD,
	}
}

// newAuditFailedEvent creates an audit event for task failure.
func newAuditFailedEvent(taskID string, durationMS int64) AuditEvent {
	return AuditEvent{
		TaskID:     taskID,
		Event:      "failed",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		DurationMS: durationMS,
	}
}
