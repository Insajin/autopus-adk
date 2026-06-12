package worker

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// recordWorkerSafetyEvent: nil WorkerLoop must not panic.
func TestRecordWorkerSafetyEvent_NilLoop(t *testing.T) {
	t.Parallel()

	assert.NotPanics(t, func() {
		recordWorkerSafetyEvent(nil, AuditEvent{TaskID: "t1", Event: "degraded"})
	})
}

// recordWorkerSafetyEvent: WorkerLoop with nil auditWriter must not panic.
func TestRecordWorkerSafetyEvent_NilAuditWriter(t *testing.T) {
	t.Parallel()

	wl := NewWorkerLoop(LoopConfig{})
	wl.auditWriter = nil

	assert.NotPanics(t, func() {
		recordWorkerSafetyEvent(wl, AuditEvent{TaskID: "t1", Event: "degraded"})
	})
}

// recordWorkerSafetyEvent: writes JSON event when auditWriter and logger are set.
func TestRecordWorkerSafetyEvent_WritesEvent(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := &testLogBuffer{}
	evt := AuditEvent{TaskID: "t2", Event: "reclaimed"}

	// Drive the code path via the wrapper function directly.
	recordAuditEvent(&buf, evt, logger)

	line := buf.String()
	assert.Contains(t, line, `"task_id":"t2"`)
	assert.Contains(t, line, `"event":"reclaimed"`)
}
