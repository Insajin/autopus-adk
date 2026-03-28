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
	regexp.MustCompile(`(?m)^>\s*$`),         // claude, gemini, opencode default prompt
	regexp.MustCompile(`(?m)^codex>\s*$`),     // codex prompt (legacy)
	regexp.MustCompile(`(?m)^\$\s*$`),         // shell $ prompt
	regexp.MustCompile(`(?m)^#\s*$`),          // root # prompt
	regexp.MustCompile(`(?m)^>\s*claude:\s*`), // claude: ready variant
}

// cliNoisePatterns matches provider CLI lines that are pure noise (used for line-level filtering).
var cliNoisePatterns = []*regexp.Regexp{
	// gemini CLI noise (line-level)
	regexp.MustCompile(`(?i)We're making changes to Gemini CLI`),
	regexp.MustCompile(`(?i)Update successful`),
	regexp.MustCompile(`(?i)What's\s+Changing:`),
	regexp.MustCompile(`(?i)How it\s+affects`),
	regexp.MustCompile(`(?i)Read more:\s*https://`),
	regexp.MustCompile(`(?i)/auth\s*$`),
	regexp.MustCompile(`(?i)/upgrade\s*$`),
	regexp.MustCompile(`(?i)Signed in with`),
	regexp.MustCompile(`(?i)Plan: Gemini`),
	// gemini CLI box drawing fragments
	regexp.MustCompile(`^[╭╰│╮╯─]+$`),
	// opencode TUI noise
	regexp.MustCompile(`(?i)Build\s+·\s+gpt`),
	regexp.MustCompile(`(?i)^\s*Build\s+GPT-[\d.]+\s+OpenAI`),
	regexp.MustCompile(`(?i)⬝+\s+esc`),
	regexp.MustCompile(`(?i)ctrl\+[a-z]\s`),
	// cmux status bar fragments
	regexp.MustCompile(`🐙\s+v?\d+\.\d+`),
}

// inlineNoisePatterns are stripped via regex replace (not line-level) to handle noise
// concatenated with content on the same line (e.g., "MCP issues detected.I will begin...").
var inlineNoisePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)MCP issues detected\.\s*Run /mcp list for status\.?`),
	regexp.MustCompile(`(?i)ℹ\s*MCP issues detected\.\s*Run\s+/mcp list\s+for\s+status\.?`),
	regexp.MustCompile(`(?i)ℹ\s*Update\s+successful!\s*The new\s+version will be used on your next run\.?`),
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

// isPromptLine checks if a single line matches any known prompt or CLI noise pattern.
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
	for _, p := range cliNoisePatterns {
		if p.MatchString(trimmed) {
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

// stripInlineNoise removes noise fragments that may be concatenated with content on the same line.
func stripInlineNoise(s string) string {
	for _, p := range inlineNoisePatterns {
		s = p.ReplaceAllString(s, "")
	}
	return s
}

// cleanScreenOutput strips ANSI codes, inline noise, and prompt lines from raw screen content.
// Used to produce clean text for merge logic (R10).
func cleanScreenOutput(raw string) string {
	cleaned := SanitizeScreenOutput(raw)
	cleaned = stripInlineNoise(cleaned)
	return filterPromptLines(cleaned)
}
