package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPipelineLogger_LogEvent_WritesJSONL verifies that LogEvent writes
// a JSONL entry to the .jsonl log file.
func TestPipelineLogger_LogEvent_WritesJSONL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logger, err := NewPipelineLogger(dir)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	event := Event{
		Type:      EventPhaseStart,
		Timestamp: time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC),
		Phase:     "phase1",
		Message:   "starting",
	}

	err = logger.LogEvent(event)
	require.NoError(t, err)

	// Read JSONL file and verify content.
	jsonlPath := filepath.Join(dir, "pipeline.jsonl")
	data, readErr := os.ReadFile(jsonlPath)
	require.NoError(t, readErr, "JSONL log file should exist at %s", jsonlPath)
	assert.Contains(t, string(data), "phase_start",
		"JSONL log should contain the event type")
}

// TestPipelineLogger_LogEvent_WritesText verifies that LogEvent writes
// a formatted text entry to the .log text file.
func TestPipelineLogger_LogEvent_WritesText(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logger, err := NewPipelineLogger(dir)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	event := Event{
		Type:    EventAgentSpawn,
		Phase:   "phase2",
		Agent:   "executor-1",
		Message: "agent started",
	}

	err = logger.LogEvent(event)
	require.NoError(t, err)

	// Read text log and verify formatted content.
	textPath := filepath.Join(dir, "pipeline.log")
	data, readErr := os.ReadFile(textPath)
	require.NoError(t, readErr, "text log file should exist at %s", textPath)
	content := string(data)
	assert.Contains(t, content, "agent started",
		"text log should contain the message")
	assert.Contains(t, content, "executor-1",
		"text log should contain the agent name")
}

// TestPipelineLogger_RoleColor verifies each role returns a correct ANSI color.
func TestPipelineLogger_RoleColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		role           string
		expectNonEmpty bool
	}{
		{"planner", true},
		{"executor", true},
		{"tester", true},
		{"reviewer", true},
		{"unknown-role", false},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			t.Parallel()
			color := RoleColor(tt.role)
			if tt.expectNonEmpty {
				assert.NotEmpty(t, color,
					"RoleColor(%q) should return a non-empty ANSI code", tt.role)
				assert.True(t, strings.HasPrefix(color, "\033["),
					"RoleColor(%q) should return an ANSI escape sequence", tt.role)
			}
		})
	}
}

// TestPipelineLogger_PromptInjection verifies the formatted prompt section
// includes the log path.
func TestPipelineLogger_PromptInjection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logger, err := NewPipelineLogger(dir)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	prompt := logger.PromptInjection()
	assert.NotEmpty(t, prompt,
		"PromptInjection should return a non-empty string")
	assert.Contains(t, prompt, dir,
		"PromptInjection should contain the log directory path")
}

// TestPipelineLogger_Close verifies that Close releases file handles.
func TestPipelineLogger_Close(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logger, err := NewPipelineLogger(dir)
	require.NoError(t, err)

	err = logger.Close()
	assert.NoError(t, err, "Close should not return error")

	// After Close, LogEvent should fail or be a no-op — but must not panic.
	assert.NotPanics(t, func() {
		_ = logger.LogEvent(Event{Type: EventError, Message: "after close"})
	}, "LogEvent after Close must not panic")
}

// TestPipelineLogger_WriteFailure_NoError verifies R9: log write failure
// should not propagate as an error to the caller.
func TestPipelineLogger_WriteFailure_NoError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logger, err := NewPipelineLogger(dir)
	require.NoError(t, err)

	// Close file handles to force write failures.
	_ = logger.Close()

	event := Event{Type: EventError, Message: "test write failure"}
	logErr := logger.LogEvent(event)
	// R9: write failures must not propagate.
	assert.NoError(t, logErr,
		"LogEvent should not return error on write failure (R9)")
}

// TestPipelineLogger_RoleColor_AllRoles verifies all mapped roles return
// correct ANSI codes and unknown roles return empty string.
func TestPipelineLogger_RoleColor_AllRoles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		role     string
		expected string
	}{
		{"lead", "\033[36m"},
		{"planner", "\033[36m"},
		{"builder", "\033[32m"},
		{"executor", "\033[32m"},
		{"tester", "\033[33m"},
		{"guardian", "\033[31m"},
		{"reviewer", "\033[31m"},
		{"auditor", "\033[35m"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, RoleColor(tt.role),
				"RoleColor(%q) mismatch", tt.role)
		})
	}
}

// TestPipelineLogger_FormatTextLine verifies text log formatting.
func TestPipelineLogger_FormatTextLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		event   Event
		wantSub string
	}{
		{
			name: "with agent",
			event: Event{
				Phase: "phase2", Agent: "executor-1", Message: "started",
			},
			wantSub: "executor-1",
		},
		{
			name: "without agent uses dash",
			event: Event{
				Phase: "phase1", Agent: "", Message: "begin",
			},
			wantSub: "[-]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			line := formatTextLine(tt.event)
			assert.Contains(t, line, tt.wantSub)
			assert.Contains(t, line, tt.event.Message)
			assert.True(t, strings.HasSuffix(line, "\n"),
				"text line should end with newline")
		})
	}
}

// TestNewPipelineLogger_InvalidDir verifies constructor error on unwritable path.
func TestNewPipelineLogger_InvalidDir(t *testing.T) {
	t.Parallel()

	// Use /dev/null/impossible — cannot create subdirectory under a device file.
	_, err := NewPipelineLogger("/dev/null/impossible")
	require.Error(t, err, "NewPipelineLogger should fail for invalid directory")
	assert.Contains(t, err.Error(), "create log dir")
}

// TestNewPipelineLogger_CreatesFiles verifies that both log files are created.
func TestNewPipelineLogger_CreatesFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logger, err := NewPipelineLogger(dir)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	_, err = os.Stat(filepath.Join(dir, "pipeline.jsonl"))
	assert.NoError(t, err, "pipeline.jsonl should be created")

	_, err = os.Stat(filepath.Join(dir, "pipeline.log"))
	assert.NoError(t, err, "pipeline.log should be created")
}

// TestPipelineLogger_Close_DoubleClose verifies double close is safe.
func TestPipelineLogger_Close_DoubleClose(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logger, err := NewPipelineLogger(dir)
	require.NoError(t, err)

	require.NoError(t, logger.Close())
	// Second close should not panic or error (files already nil).
	assert.NoError(t, logger.Close(), "double Close should not error")
}

// TestPipelineLogger_LogEvent_MultipleEvents verifies sequential log writes.
func TestPipelineLogger_LogEvent_MultipleEvents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logger, err := NewPipelineLogger(dir)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	events := []Event{
		{Type: EventPhaseStart, Phase: "phase1", Message: "start"},
		{Type: EventAgentSpawn, Phase: "phase2", Agent: "ex-1", Message: "spawn"},
		{Type: EventPhaseEnd, Phase: "phase1", Message: "end"},
	}

	for _, e := range events {
		require.NoError(t, logger.LogEvent(e))
	}

	// Verify JSONL has 3 lines.
	data, err := os.ReadFile(filepath.Join(dir, "pipeline.jsonl"))
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 3, "should have 3 JSONL lines")
}
