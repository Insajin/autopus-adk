package orchestra

import (
	"os"
	"regexp"
	"strings"
	"time"
)

// ansiEscapeRe matches ANSI escape sequences including color codes, cursor movement, etc.
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes all ANSI escape sequences from the input string.
func stripANSI(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

// defaultPromptPatterns matches common shell and CLI prompts.
// @AX:NOTE [AUTO] hardcoded prompt regexes — must stay in sync with DefaultCompletionPatterns
var defaultPromptPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^>\s*$`),         // claude, gemini default prompt
	regexp.MustCompile(`(?m)^codex>\s*$`),     // codex prompt
	regexp.MustCompile(`(?m)^\$\s*$`),         // shell $ prompt
	regexp.MustCompile(`(?m)^#\s*$`),          // root # prompt
	regexp.MustCompile(`(?m)^>\s*claude:\s*`), // claude: ready variant
}

// filterPromptLines removes lines matching known CLI prompt patterns from output.
func filterPromptLines(output string) string {
	lines := strings.Split(output, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if isPromptLine(line) {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

// isPromptLine checks if a single line matches any known prompt pattern.
func isPromptLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	for _, p := range defaultPromptPatterns {
		if p.MatchString(line) {
			return true
		}
	}
	return false
}

// isPromptVisible checks if the screen content contains a visible prompt pattern,
// indicating the CLI session has returned to input-ready state.
// This is the PRIMARY completion detection method (R7).
// @AX:NOTE [AUTO] called by pollUntilPrompt and waitForCompletion — central prompt detection logic
func isPromptVisible(screen string, patterns []CompletionPattern) bool {
	// Check provider-specific patterns first
	for _, cp := range patterns {
		if cp.Pattern.MatchString(screen) {
			return true
		}
	}
	// Fallback to default patterns
	for _, p := range defaultPromptPatterns {
		if p.MatchString(screen) {
			return true
		}
	}
	return false
}

// isOutputIdle checks if the output file has not been modified for the given threshold.
// This is the SECONDARY completion detection method (R7).
func isOutputIdle(outputFile string, threshold time.Duration) bool {
	info, err := os.Stat(outputFile)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) >= threshold
}

// cleanScreenOutput strips ANSI codes and filters prompt lines from raw screen content.
// Used to produce clean text for merge logic (R10).
func cleanScreenOutput(raw string) string {
	cleaned := stripANSI(raw)
	return filterPromptLines(cleaned)
}
