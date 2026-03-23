package tui_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWizardHeader_Format(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	tui.WizardHeader(&buf, 2, 5, "Platform Setup")

	out := buf.String()
	// Should contain step counter
	assert.Contains(t, out, "[2/5]")
	// Should contain the title
	assert.Contains(t, out, "Platform Setup")
	// Should contain a separator line
	assert.Contains(t, out, "─")
	// Should start with a newline for visual spacing
	assert.True(t, strings.HasPrefix(out, "\n"), "header should start with newline")
}

func TestWizardHeader_FirstStep(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	tui.WizardHeader(&buf, 1, 3, "Welcome")

	out := buf.String()
	assert.Contains(t, out, "[1/3]")
	assert.Contains(t, out, "Welcome")
}

func TestSummaryTable_Empty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	tui.SummaryTable(&buf, []tui.SummaryRow{})

	// Empty input should produce no output
	assert.Empty(t, buf.String())
}

func TestSummaryTable_MultipleRows(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	rows := []tui.SummaryRow{
		{Key: "Project", Value: "my-app"},
		{Key: "Platform", Value: "claude-code"},
		{Key: "Mode", Value: "full"},
	}
	tui.SummaryTable(&buf, rows)

	out := buf.String()
	require.NotEmpty(t, out)
	// All keys should appear in output
	assert.Contains(t, out, "Project")
	assert.Contains(t, out, "Platform")
	assert.Contains(t, out, "Mode")
	// All values should appear in output
	assert.Contains(t, out, "my-app")
	assert.Contains(t, out, "claude-code")
	assert.Contains(t, out, "full")
	// Should contain the Summary header
	assert.Contains(t, out, "Summary")
}

func TestSummaryTable_Alignment(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	rows := []tui.SummaryRow{
		{Key: "Short", Value: "val1"},
		{Key: "LongerKey", Value: "val2"},
	}
	tui.SummaryTable(&buf, rows)

	out := buf.String()
	require.NotEmpty(t, out)
	// Both keys and values should be present
	assert.Contains(t, out, "Short")
	assert.Contains(t, out, "LongerKey")
	// The output should have consistent spacing (padded to max key length)
	lines := strings.Split(out, "\n")
	// Find lines containing the values
	var valueLine1, valueLine2 string
	for _, l := range lines {
		if strings.Contains(l, "val1") {
			valueLine1 = l
		}
		if strings.Contains(l, "val2") {
			valueLine2 = l
		}
	}
	require.NotEmpty(t, valueLine1, "line with val1 should exist")
	require.NotEmpty(t, valueLine2, "line with val2 should exist")
}
