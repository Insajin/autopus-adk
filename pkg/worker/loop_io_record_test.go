package worker

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingWriter always returns an error to exercise the audit failure path.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("disk full")
}

func TestRecordAuditEvent_WritesJSONLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := &testLogBuffer{}
	evt := AuditEvent{TaskID: "task-io", Event: "completed", DurationMS: 42}

	recordAuditEvent(&buf, evt, logger)

	line := buf.String()
	require.True(t, strings.HasSuffix(line, "\n"), "audit line must be newline-terminated")
	assert.Contains(t, line, `"task_id":"task-io"`)
	assert.Contains(t, line, `"event":"completed"`)
	assert.Empty(t, logger.warnings, "successful write records no warning")
}

func TestRecordAuditEvent_FailureRecordsWarning(t *testing.T) {
	t.Parallel()

	logger := &testLogBuffer{}
	evt := AuditEvent{TaskID: "task-fail", Event: "failed"}

	// recordAuditEvent must not panic even when the writer fails.
	recordAuditEvent(failingWriter{}, evt, logger)

	require.Len(t, logger.warnings, 1)
	assert.Contains(t, logger.warnings[0], "task-fail")
	assert.Contains(t, logger.warnings[0], "disk full")
}
