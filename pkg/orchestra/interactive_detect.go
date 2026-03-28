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
	regexp.MustCompile(`(?m)Ask anything`),        // opencode TUI input placeholder
	regexp.MustCompile(`(?m)^❯\s*$`),              // claude code prompt (unicode heavy right-pointing angle)
	regexp.MustCompile(`(?m)^\s*>\s+(Type your|@)`), // gemini TUI prompt (> Type your message...)
	regexp.MustCompile(`(?m)^codex>\s*$`),         // codex prompt (legacy)
	regexp.MustCompile(`(?m)^\$\s*$`),             // shell $ prompt
	regexp.MustCompile(`(?m)^#\s*$`),              // root # prompt
}

// sessionReadyPromptPatterns matches CLI-specific prompts WITHOUT shell patterns ($ and #).
// Used by waitForSessionReady to avoid premature detection on bare shell prompts.
var sessionReadyPromptPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)Ask anything`),          // opencode TUI input placeholder
	regexp.MustCompile(`(?m)^❯\s*$`),                // claude code prompt (unicode heavy right-pointing angle)
	regexp.MustCompile(`(?m)^\s*>\s+(Type your|@)`),  // gemini TUI prompt
	regexp.MustCompile(`(?m)^codex>\s*$`),           // codex prompt (legacy)
	// NOTE: no shell $ or # patterns — this is the key difference from defaultPromptPatterns
}

// SessionReadyPatterns returns completion patterns for CLI session readiness detection.
// Unlike DefaultCompletionPatterns, this excludes shell prompts ($ and #) to prevent
// false positives when detecting whether a CLI tool has finished launching.
func SessionReadyPatterns() []CompletionPattern {
	return []CompletionPattern{
		{Provider: "claude", Pattern: regexp.MustCompile(`(?m)^❯\s*$`)},
		{Provider: "codex", Pattern: regexp.MustCompile(`(?m)^codex>\s*$`)},
		{Provider: "gemini", Pattern: regexp.MustCompile(`(?m)^\s*>\s+(Type your|@)`)},
		{Provider: "opencode", Pattern: regexp.MustCompile(`(?m)Ask anything`)},
	}
}

// isSessionReady checks if the screen content contains a CLI-specific prompt pattern,
// indicating the provider session has fully launched. Unlike isPromptVisible, this does
// NOT match shell prompts ($ and #) to avoid false positives during startup.
func isSessionReady(screen string, patterns []CompletionPattern) bool {
	screen = stripANSI(screen)
	// Check provider-specific session-ready patterns
	for _, cp := range patterns {
		if cp.Pattern.MatchString(screen) {
			return true
		}
	}
	// Fallback to sessionReadyPromptPatterns (no shell patterns)
	for _, p := range sessionReadyPromptPatterns {
		if p.MatchString(screen) {
			return true
		}
	}
	return false
}

// startupTimeoutFor returns the per-provider startup timeout.
// opencode loads plugins/MCP on startup similarly to claude, requiring a longer timeout.
func startupTimeoutFor(provider ProviderConfig) time.Duration {
	if provider.StartupTimeout > 0 {
		return provider.StartupTimeout
	}
	switch provider.Name {
	case "claude":
		return 15 * time.Second
	case "gemini":
		return 10 * time.Second
	case "opencode":
		return 15 * time.Second
	default:
		return 30 * time.Second
	}
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
	// gemini CLI box drawing and single-char wrapped lines
	regexp.MustCompile(`^[╭╰│╮╯─]+$`),
	regexp.MustCompile(`^│\s*.{1,3}\s*│$`),
	regexp.MustCompile(`(?i)^Positional arguments now default`),
	regexp.MustCompile(`(?i)non-interactive mode.*--prompt`),
	// opencode TUI noise
	regexp.MustCompile(`(?i)Build\s+·\s+gpt`),
	regexp.MustCompile(`(?i)^\s*Build\s+GPT-[\d.]+\s+OpenAI`),
	regexp.MustCompile(`(?i)⬝+\s+esc`),
	regexp.MustCompile(`(?i)ctrl\+[a-z]\s`),
	// Additional opencode TUI chrome (without "Build" prefix)
	regexp.MustCompile(`(?i)^\s*gpt-[\d.]+\s+OpenAI`),
	// Shell login banner (macOS/Linux)
	regexp.MustCompile(`(?i)^Last login:`),
	// User@host shell prompt (zsh %, bash $, root #)
	regexp.MustCompile(`^\w+@[\w.-]+.*[%$#]\s*$`),
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
	// Strip ANSI escape codes before matching — providers like claude/opencode
	// render the ">" prompt with color codes (e.g. \x1b[32m>\x1b[0m) that break
	// the ^>\s*$ pattern when matching raw ReadScreen output.
	screen = stripANSI(screen)
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
