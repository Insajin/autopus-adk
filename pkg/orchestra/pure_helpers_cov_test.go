package orchestra

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRedactSensitiveText_AllPatterns exercises each sensitive-pattern branch in
// redactSensitiveText and asserts the secret value is gone while the label is kept.
func TestRedactSensitiveText_AllPatterns(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		input      string
		mustGone   string
		mustRemain string
	}{
		{"authorization bearer header", "Authorization: Bearer abc123tokenXYZ", "abc123tokenXYZ", "Authorization"},
		{"api key assignment", "OPENAI_API_KEY=sk-supersecretvalue", "supersecretvalue", "OPENAI_API_KEY"},
		{"token colon form", "token: mytokenvalue123", "mytokenvalue123", "token"},
		{"password equals form", "password=hunter2secret", "hunter2secret", "password"},
		{"bare bearer", "bearer abcdef.ghij-klmn", "abcdef.ghij-klmn", ""},
		{"openai sk prefix", "key is sk-AbCdEf0123456789 here", "sk-AbCdEf0123456789", "key is"},
		{"jwt token", "jwt eyJhbGciOiJIUzI1.eyJzdWIiOiIx.SflKxwRJSM", "SflKxwRJSM", "jwt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactSensitiveText(tc.input)
			assert.NotContains(t, got, tc.mustGone, "secret value should be redacted")
			assert.Contains(t, got, "***", "redaction marker should appear")
			if tc.mustRemain != "" {
				assert.Contains(t, got, tc.mustRemain, "label should be preserved")
			}
		})
	}
}

// TestRedactSensitiveText_NoSecret verifies benign text passes through unchanged.
func TestRedactSensitiveText_NoSecret(t *testing.T) {
	t.Parallel()
	in := "this is a normal log line with no secrets"
	assert.Equal(t, in, redactSensitiveText(in))
}

// TestSafePreview_Truncation covers both the short (no truncation) and long
// (truncated with ellipsis) branches of safePreview.
func TestSafePreview_Truncation(t *testing.T) {
	t.Parallel()
	short := safePreview("  hello   world  ", 120)
	assert.Equal(t, "hello world", short, "whitespace collapsed, no truncation")

	long := safePreview(strings.Repeat("a", 200), 10)
	assert.Equal(t, strings.Repeat("a", 10)+"...", long)
	assert.Len(t, long, 13)
}

// TestExtractListItem_Formats covers the backtick, bold, plain-dash, and
// non-list branches of extractListItem.
func TestExtractListItem_Formats(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"- `cacheManager`", "cacheManager"},
		{"- **AuthService**", "AuthService"},
		{"- plain text only", ""},
		{"not a list line", ""},
		{"- `unterminated", ""},
		{"- **unterminated", ""},
	}
	for _, tc := range cases {
		assert.Equalf(t, tc.want, extractListItem(tc.in), "input=%q", tc.in)
	}
}

// TestLastPromptArgIndex finds the last matching prompt arg and returns -1 when absent.
func TestLastPromptArgIndex(t *testing.T) {
	t.Parallel()
	args := []string{"--flag", "P", "--other", "P"}
	assert.Equal(t, 3, lastPromptArgIndex(args, "P"), "should return last matching index")
	assert.Equal(t, -1, lastPromptArgIndex(args, "missing"))
	assert.Equal(t, -1, lastPromptArgIndex(nil, "x"))
}

// TestInsertArgs inserts values at an index and clamps out-of-range indices.
func TestInsertArgs(t *testing.T) {
	t.Parallel()
	base := []string{"a", "b", "c"}
	assert.Equal(t, []string{"a", "X", "Y", "b", "c"}, insertArgs(base, 1, "X", "Y"))
	// Negative index clamps to append at end.
	assert.Equal(t, []string{"a", "b", "c", "Z"}, insertArgs(base, -5, "Z"))
	// Index beyond length clamps to append at end.
	assert.Equal(t, []string{"a", "b", "c", "W"}, insertArgs(base, 99, "W"))
}

// TestAppendSubprocessDiagnostic covers empty-existing, empty-diagnostic, and
// join branches.
func TestAppendSubprocessDiagnostic(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "first", appendSubprocessDiagnostic("", "  first  "))
	assert.Equal(t, "kept", appendSubprocessDiagnostic("kept", "   "))
	assert.Equal(t, "a\nb", appendSubprocessDiagnostic("a", "b"))
	assert.Equal(t, "  ", appendSubprocessDiagnostic("  ", ""))
}

// TestIsProviderStillWorking exercises every branch: no patterns (false),
// pattern match (true), and patterns present but none matching (false).
func TestIsProviderStillWorking(t *testing.T) {
	t.Parallel()

	// No patterns configured — always false regardless of screen content.
	assert.False(t, isProviderStillWorking("generating output...", nil))
	assert.False(t, isProviderStillWorking("generating output...", []string{}))

	// Pattern present and found in screen content — still working.
	assert.True(t, isProviderStillWorking("thinking...", []string{"thinking"}))

	// Patterns present but none match — provider is idle.
	assert.False(t, isProviderStillWorking("idle screen", []string{"thinking", "generating"}))
}

// TestIsProviderStillWorking_ANSIStripped verifies ANSI escape codes are
// stripped before pattern matching so patterns work on clean text.
func TestIsProviderStillWorking_ANSIStripped(t *testing.T) {
	t.Parallel()
	// ANSI color codes wrap the keyword; stripANSI must clean it first.
	ansiScreen := "\033[1;32mStreaming\033[0m response..."
	assert.True(t, isProviderStillWorking(ansiScreen, []string{"Streaming"}))
}
