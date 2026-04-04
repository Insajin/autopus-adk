package worker

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/audit"
)

// AuditEvent represents a structured audit log entry for task execution.
type AuditEvent struct {
	TaskID      string  `json:"task_id"`
	Event       string  `json:"event"` // "started", "completed", "failed"
	Timestamp   string  `json:"timestamp"`
	DurationMS  int64   `json:"duration_ms,omitempty"`
	CostUSD     float64 `json:"cost_usd,omitempty"`
	ComputerUse bool    `json:"computer_use,omitempty"`
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
func newAuditStartedEvent(taskID string, computerUse bool) AuditEvent {
	return AuditEvent{
		TaskID:      taskID,
		Event:       "started",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		ComputerUse: computerUse,
	}
}

// newAuditCompletedEvent creates an audit event for task completion.
func newAuditCompletedEvent(taskID string, durationMS int64, costUSD float64, computerUse bool) AuditEvent {
	return AuditEvent{
		TaskID:      taskID,
		Event:       "completed",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		DurationMS:  durationMS,
		CostUSD:     costUSD,
		ComputerUse: computerUse,
	}
}

// newAuditFailedEvent creates an audit event for task failure.
func newAuditFailedEvent(taskID string, durationMS int64, computerUse bool) AuditEvent {
	return AuditEvent{
		TaskID:      taskID,
		Event:       "failed",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		DurationMS:  durationMS,
		ComputerUse: computerUse,
	}
}

// LogBuffer captures structured log entries for audit write error tracking.
// Implementations record warnings and errors for consecutive failure escalation.
type LogBuffer interface {
	RecordWarning(msg string)
	RecordError(msg string)
}

// auditFailCounter tracks consecutive audit write failures.
// Reset to 0 on successful write. Escalates at threshold.
type auditFailCounter struct {
	count     int
	threshold int
}

// newAuditFailCounter creates a counter with the given escalation threshold.
func newAuditFailCounter(threshold int) *auditFailCounter {
	return &auditFailCounter{threshold: threshold}
}

// recordResult updates the counter based on write success/failure.
// Returns true if escalation threshold was just reached.
func (c *auditFailCounter) recordResult(err error) bool {
	if err == nil {
		c.count = 0
		return false
	}
	c.count++
	return c.count == c.threshold
}

// writeResilientAuditEvent wraps audit event writing with error logging and
// consecutive failure escalation. Audit write failures never affect task
// execution. SPEC-WORKER-003 REQ-AUDIT-01/02.
func writeResilientAuditEvent(w io.Writer, evt AuditEvent, logger LogBuffer) error {
	data, err := json.Marshal(evt)
	if err != nil {
		msg := fmt.Sprintf("audit marshal error: task_id=%s event=%s err=%v", evt.TaskID, evt.Event, err)
		logger.RecordWarning(msg)
		slog.Warn("audit write failed", "task_id", evt.TaskID, "event", evt.Event, "error", err)
		return nil
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	if err == nil {
		return nil
	}

	// Consecutive failure tracking via logger.
	// The LogBuffer implementation tracks consecutive failures internally.
	msg := fmt.Sprintf("audit write failed: task_id=%s event=%s err=%v", evt.TaskID, evt.Event, err)
	logger.RecordWarning(msg)
	slog.Warn("audit write failed", "task_id", evt.TaskID, "event", evt.Event, "error", err)

	return nil
}

// slogAuditLogger is the production LogBuffer that uses slog for structured
// logging and tracks consecutive failures for escalation.
type slogAuditLogger struct {
	consecutiveFailures int
	threshold           int
}

// newSlogAuditLogger creates a production audit logger with the given threshold.
func newSlogAuditLogger(threshold int) *slogAuditLogger {
	return &slogAuditLogger{threshold: threshold}
}

// RecordWarning logs a warning and escalates if consecutive failures reach threshold.
func (l *slogAuditLogger) RecordWarning(msg string) {
	l.consecutiveFailures++
	if l.consecutiveFailures >= l.threshold {
		l.RecordError(msg)
		return
	}
	slog.Warn(msg)
}

// RecordError logs an error-level message.
func (l *slogAuditLogger) RecordError(msg string) {
	slog.Error("audit write: consecutive failures reached threshold", "detail", msg,
		"consecutive_failures", l.consecutiveFailures)
}

// Reset resets the consecutive failure counter on successful write.
func (l *slogAuditLogger) Reset() {
	l.consecutiveFailures = 0
}
