// Package worker - edge case tests for audit event functions and failure tracking.
package worker

import (
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewAuditStartedEvent verifies the started event constructor populates fields.
func TestNewAuditStartedEvent(t *testing.T) {
	t.Parallel()

	evt := newAuditStartedEvent("task-1", true)

	assert.Equal(t, "task-1", evt.TaskID)
	assert.Equal(t, "started", evt.Event)
	assert.NotEmpty(t, evt.Timestamp, "Timestamp should be set")
	assert.True(t, evt.ComputerUse, "ComputerUse should be true")
}

// TestNewAuditCompletedEvent verifies the completed event constructor.
func TestNewAuditCompletedEvent(t *testing.T) {
	t.Parallel()

	evt := newAuditCompletedEvent("task-2", 1500, 0.05, false)

	assert.Equal(t, "task-2", evt.TaskID)
	assert.Equal(t, "completed", evt.Event)
	assert.Equal(t, int64(1500), evt.DurationMS)
	assert.Equal(t, 0.05, evt.CostUSD)
	assert.False(t, evt.ComputerUse)
}

// TestNewAuditFailedEvent verifies the failed event constructor.
func TestNewAuditFailedEvent(t *testing.T) {
	t.Parallel()

	evt := newAuditFailedEvent("task-3", 3000, true)

	assert.Equal(t, "task-3", evt.TaskID)
	assert.Equal(t, "failed", evt.Event)
	assert.Equal(t, int64(3000), evt.DurationMS)
	assert.True(t, evt.ComputerUse)
}

// TestWriteAuditEvent_NilWriter returns nil (audit disabled).
func TestWriteAuditEvent_NilWriter(t *testing.T) {
	t.Parallel()

	err := writeAuditEvent(nil, AuditEvent{TaskID: "task-nil"})
	assert.NoError(t, err, "nil writer should return nil (audit disabled)")
}

// TestWriteAuditEvent_ValidWriter writes JSON Lines correctly.
func TestWriteAuditEvent_ValidWriter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w, err := audit.NewRotatingWriter(dir+"/audit.log", 1024*1024, 0)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, w.Close())
	}()

	evt := AuditEvent{
		TaskID:      "task-write",
		Event:       "completed",
		DurationMS:  100,
		ComputerUse: true,
	}

	err = writeAuditEvent(w, evt)
	assert.NoError(t, err)

	// Verify the data was serialized as JSON.
	data, _ := json.Marshal(evt)
	assert.Contains(t, string(data), `"task_id":"task-write"`)
}

// TestAuditFailCounter_RecordResult covers the counter logic.
func TestAuditFailCounter_RecordResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		threshold int
		errors    []error
		wantEsc   []bool
	}{
		{
			name:      "no errors resets count",
			threshold: 3,
			errors:    []error{nil, nil, nil},
			wantEsc:   []bool{false, false, false},
		},
		{
			name:      "3 consecutive errors triggers escalation",
			threshold: 3,
			errors:    []error{errTest, errTest, errTest},
			wantEsc:   []bool{false, false, true},
		},
		{
			name:      "success resets counter mid-stream",
			threshold: 3,
			errors:    []error{errTest, errTest, nil, errTest, errTest, errTest},
			wantEsc:   []bool{false, false, false, false, false, true},
		},
		{
			name:      "threshold 1 escalates immediately",
			threshold: 1,
			errors:    []error{errTest},
			wantEsc:   []bool{true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newAuditFailCounter(tt.threshold)
			for i, err := range tt.errors {
				escalated := c.recordResult(err)
				assert.Equal(t, tt.wantEsc[i], escalated,
					"step %d: escalation mismatch", i)
			}
		})
	}
}

// TestSlogAuditLogger_RecordWarning_Escalation covers the slog logger.
func TestSlogAuditLogger_RecordWarning_Escalation(t *testing.T) {
	t.Parallel()

	logger := newSlogAuditLogger(3)

	// First 2 warnings: no escalation.
	logger.RecordWarning("fail 1")
	logger.RecordWarning("fail 2")
	assert.Equal(t, 2, logger.consecutiveFailures)

	// Third warning: escalates to error.
	logger.RecordWarning("fail 3")
	assert.Equal(t, 3, logger.consecutiveFailures)
}

// TestSlogAuditLogger_Reset clears the counter.
func TestSlogAuditLogger_Reset(t *testing.T) {
	t.Parallel()

	logger := newSlogAuditLogger(3)
	logger.RecordWarning("fail")
	assert.Equal(t, 1, logger.consecutiveFailures)

	logger.Reset()
	assert.Equal(t, 0, logger.consecutiveFailures)
}

// TestSlogAuditLogger_RecordError directly logs error.
func TestSlogAuditLogger_RecordError(t *testing.T) {
	t.Parallel()

	logger := newSlogAuditLogger(3)
	// Should not panic.
	logger.RecordError("direct error")
}

// TestWriteResilientAuditEvent_SuccessfulWrite returns nil silently.
func TestWriteResilientAuditEvent_SuccessfulWrite(t *testing.T) {
	t.Parallel()

	w := &successWriter{}
	logBuf := &testLogBuffer{}

	evt := AuditEvent{TaskID: "ok-1", Event: "completed"}
	err := writeResilientAuditEvent(w, evt, logBuf)

	assert.NoError(t, err)
	assert.False(t, logBuf.hasWarning(), "no warning on successful write")
}

// --- Test helpers ---

var errTest = assert.AnError

type successWriter struct{}

func (w *successWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

// TestComputerUseSupported covers the provider check function.
func TestComputerUseSupported(t *testing.T) {
	t.Parallel()

	tests := []struct {
		provider string
		want     bool
	}{
		{"claude", true},
		{"codex", false},
		{"gemini", false},
		{"", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ComputerUseSupported(tt.provider))
		})
	}
}
