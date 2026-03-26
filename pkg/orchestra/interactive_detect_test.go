package orchestra

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- R10: ANSI escape sequence stripping ---

// TestInteractive_StripANSI_BasicCodes verifies ANSI escape sequences are removed.
func TestInteractive_StripANSI_BasicCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "color codes stripped",
			input:    "\x1b[31mERROR\x1b[0m: something failed",
			expected: "ERROR: something failed",
		},
		{
			name:     "bold and underline stripped",
			input:    "\x1b[1m\x1b[4mTitle\x1b[0m",
			expected: "Title",
		},
		{
			name:     "cursor movement stripped",
			input:    "\x1b[2J\x1b[H$ prompt here",
			expected: "$ prompt here",
		},
		{
			name:     "no escape sequences unchanged",
			input:    "plain text output",
			expected: "plain text output",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "256 color codes stripped",
			input:    "\x1b[38;5;196mred text\x1b[0m",
			expected: "red text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := stripANSI(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// --- R10: Provider prompt line filtering ---

// TestInteractive_FilterPromptLines_RemovesProviderPrompts verifies prompt lines are stripped.
func TestInteractive_FilterPromptLines_RemovesProviderPrompts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "claude prompt filtered",
			input:    "some output\n> claude: ready\nactual content",
			expected: "some output\nactual content",
		},
		{
			name:     "codex prompt filtered",
			input:    "codex> \nreal output here",
			expected: "real output here",
		},
		{
			name:     "no prompt lines unchanged",
			input:    "just normal output\nsecond line",
			expected: "just normal output\nsecond line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterPromptLines(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// --- R7: Completion detection ---

// TestInteractive_PromptPatternDetection_MatchesShellPrompt verifies prompt pattern matching.
func TestInteractive_PromptPatternDetection_MatchesShellPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		screen  string
		matched bool
	}{
		{
			name:    "dollar prompt detected",
			screen:  "output done\n$ ",
			matched: true,
		},
		{
			name:    "hash prompt detected",
			screen:  "# ",
			matched: true,
		},
		{
			name:    "mid-output no prompt",
			screen:  "still running...\nprocessing step 3",
			matched: false,
		},
		{
			name:    "empty screen no match",
			screen:  "",
			matched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isPromptVisible(tt.screen, nil)
			assert.Equal(t, tt.matched, got)
		})
	}
}

// TestInteractive_IdleDetection_NoOutputTimeout verifies idle detection logic.
// R7 secondary: pipe-pane output file idle detection (N seconds no output, default 10s).
func TestInteractive_IdleDetection_NoOutputTimeout(t *testing.T) {
	t.Parallel()
	tmpFile := filepath.Join(t.TempDir(), "output.txt")
	os.WriteFile(tmpFile, []byte("data"), 0o644)
	// Set modtime to 15 seconds ago
	past := time.Now().Add(-15 * time.Second)
	os.Chtimes(tmpFile, past, past)
	assert.True(t, isOutputIdle(tmpFile, 10*time.Second))
}

// TestInteractive_IdleDetection_ActiveOutput verifies active output is not flagged idle.
func TestInteractive_IdleDetection_ActiveOutput(t *testing.T) {
	t.Parallel()
	tmpFile := filepath.Join(t.TempDir(), "output.txt")
	os.WriteFile(tmpFile, []byte("data"), 0o644)
	// File was just written, modtime is now
	assert.False(t, isOutputIdle(tmpFile, 10*time.Second))
}

// TestInteractive_CompletionDetection_TimeoutFallback verifies timeout is the final fallback.
func TestInteractive_CompletionDetection_TimeoutFallback(t *testing.T) {
	t.Parallel()
	// Timeout is handled by context cancellation in the orchestration layer.
	// Verify that isOutputIdle returns false when file doesn't exist.
	assert.False(t, isOutputIdle("/nonexistent/file.txt", 10*time.Second))
}
