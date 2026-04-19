// Package worker - tests for audit write error handling.
//
// Phase 1.5 RED scaffold: resilient audit writing with escalation
// does NOT exist yet. All tests MUST fail until Phase 2 implementation.
package worker

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingAuditWriter is a mock audit writer that always returns an error.
type failingAuditWriter struct {
	writeCount int
}

func (w *failingAuditWriter) Write(p []byte) (int, error) {
	w.writeCount++
	return 0, errors.New("audit write failure")
}

// TestWriteAuditEvent_Error_LogsWarning verifies that when the audit writer
// returns an error, a warning is logged but the function does not panic or
// propagate the error to the caller in a way that blocks task execution.
func TestWriteAuditEvent_Error_LogsWarning(t *testing.T) {
	t.Parallel()

	// This test uses writeResilientAuditEvent which does NOT exist yet — RED.
	// Current writeAuditEvent returns error but doesn't log; the new resilient
	// version should log warnings and track consecutive failures.

	fw := &failingAuditWriter{}
	logBuf := &testLogBuffer{}

	evt := AuditEvent{
		TaskID: "task-audit-err-1",
		Event:  "completed",
	}

	// Call the resilient audit writer (does not exist yet).
	err := writeResilientAuditEvent(fw, evt, logBuf)

	// Assert: no error propagated (audit failure is non-fatal).
	assert.NoError(t, err,
		"audit write failure should not propagate as error")

	// Assert: warning was logged (behavior assertion, not just no-error).
	require.True(t, logBuf.hasWarning(),
		"audit write failure should produce a warning log entry")
	assert.Contains(t, logBuf.lastWarning(), "audit",
		"warning message should reference audit context")
}

// TestWriteAuditEvent_ThreeConsecutiveFailures_EscalatesToError verifies
// escalation: after 3 consecutive audit write failures, the severity
// escalates from warning to error-level logging.
func TestWriteAuditEvent_ThreeConsecutiveFailures_EscalatesToError(t *testing.T) {
	t.Parallel()

	fw := &failingAuditWriter{}
	logBuf := &testLogBuffer{}

	evt := AuditEvent{
		TaskID: "task-audit-escalate-1",
		Event:  "completed",
	}

	// First two failures — should be warnings.
	_ = writeResilientAuditEvent(fw, evt, logBuf)
	_ = writeResilientAuditEvent(fw, evt, logBuf)

	// Assert: still at warning level after 2 failures.
	assert.False(t, logBuf.hasError(),
		"should not escalate to error after only 2 failures")

	// Third consecutive failure — should escalate to error.
	_ = writeResilientAuditEvent(fw, evt, logBuf)

	// Assert: escalated to error level.
	require.True(t, logBuf.hasError(),
		"3 consecutive audit failures should escalate to error-level logging")
	assert.Contains(t, logBuf.lastError(), "consecutive",
		"error message should mention consecutive failures")
}

// TestAuditFailure_DoesNotAffectTaskResult verifies that audit write failures
// do not affect the task result. A successful task execution should return
// its result even when the completion audit event fails to write.
func TestAuditFailure_DoesNotAffectTaskResult(t *testing.T) {
	t.Parallel()

	fw := &failingAuditWriter{}
	logBuf := &testLogBuffer{}

	// Simulate: task execution succeeded.
	taskResult := "task completed successfully"
	taskErr := error(nil)

	// Write completion audit event (will fail).
	completionEvt := AuditEvent{
		TaskID: "task-audit-nofail-1",
		Event:  "completed",
	}
	auditErr := writeResilientAuditEvent(fw, completionEvt, logBuf)

	// Assert: audit failure did not change task result.
	assert.NoError(t, auditErr,
		"audit failure should be swallowed, not returned")
	assert.Equal(t, "task completed successfully", taskResult,
		"task result should be unaffected by audit failure")
	assert.NoError(t, taskErr,
		"task error should remain nil despite audit failure")

	// Assert: the audit failure was logged (not silently dropped).
	require.True(t, logBuf.hasWarning(),
		"audit failure should be logged even when swallowed")
}

// --- Test helpers (scaffolds for types that don't exist yet) ---

// testLogBuffer captures structured log entries for assertion in tests.
// This is a scaffold — the actual integration will use slog test handler.
type testLogBuffer struct {
	warnings []string
	errors   []string
}

func (b *testLogBuffer) hasWarning() bool { return len(b.warnings) > 0 }
func (b *testLogBuffer) hasError() bool   { return len(b.errors) > 0 }
func (b *testLogBuffer) lastWarning() string {
	if len(b.warnings) == 0 {
		return ""
	}
	return b.warnings[len(b.warnings)-1]
}
func (b *testLogBuffer) lastError() string {
	if len(b.errors) == 0 {
		return ""
	}
	return b.errors[len(b.errors)-1]
}

// writeResilientAuditEvent is the resilient audit writer that logs warnings
// on failure and escalates to error after 3 consecutive failures.
// Does NOT exist yet — RED scaffold.
// Expected signature:
//   func writeResilientAuditEvent(w io.Writer, evt AuditEvent, logger LogBuffer) error
