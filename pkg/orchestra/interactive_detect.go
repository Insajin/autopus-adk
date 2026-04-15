package orchestra

import (
	"os"
	"regexp"
	"strings"
	"time"
)

// ansiEscapeRe matches ANSI escape sequences including color codes, cursor movement, etc.
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// iceTableHeaderRe matches ICE scoring table headers (various formats).
var iceTableHeaderRe = regexp.MustCompile(`(?i)(ICE\s*(Score|스코어)|통합\s*ICE|Top\s*\d+\s*(통합|아이디어)|Judge.*Merge|Judge.*Integration|Impact.*Confidence.*Ease)`)

// iceScoreLineRe matches standalone ICE score lines like "ICE: 5.12" or "Score: 432".
var iceScoreLineRe = regexp.MustCompile(`(?i)^\s*(ICE|Score)\s*[:=]\s*[\d.]+\s*$`)

// stripANSI removes all ANSI escape sequences from the input string.
func stripANSI(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

// defaultPromptPatterns matches common shell and CLI prompts.
// @AX:NOTE [AUTO] hardcoded prompt regexes — must stay in sync with DefaultCompletionPatterns
var defaultPromptPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^❯\s*$`),                     // claude code prompt (unicode heavy right-pointing angle)
	regexp.MustCompile(`(?m)^\s*>\s*(Type your|@|\s*$)`), // gemini TUI prompt (> Type your..., > @, bare >)
	regexp.MustCompile(`(?im)^codex>\s*$`),               // codex prompt (case-insensitive)
	regexp.MustCompile(`(?im)^Ask anything\s*$`),         // opencode TUI prompt
	regexp.MustCompile(`(?m)^\$\s*$`),                    // shell $ prompt
	regexp.MustCompile(`(?m)^#\s*$`),                     // root # prompt
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
// If a working indicator (spinner, "Thinking", etc.) is also visible, the prompt
// is considered a false positive — the provider's TUI shows the prompt at all times.
// @AX:NOTE [AUTO] called by pollUntilPrompt and waitForCompletion — central prompt detection logic
func isPromptVisible(screen string, patterns []CompletionPattern) bool {
	// Strip ANSI escape codes before matching — providers like claude/opencode
	// render the ">" prompt with color codes (e.g. \x1b[32m>\x1b[0m) that break
	// the ^>\s*$ pattern when matching raw ReadScreen output.
	screen = stripANSI(screen)

	// Guard: if working indicators are visible, the provider is still active.
	// Some TUIs (e.g., Gemini) show the idle prompt alongside a spinner.
	if isProviderWorking(screen) {
		return false
	}

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

// toolApprovalPatterns matches interactive tool permission prompts from providers.
// When detected, the orchestra auto-approves by sending "1" (Allow once).
var toolApprovalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Action Required`),                   // gemini tool permission
	regexp.MustCompile(`(?i)Allow execution of`),                // gemini sandbox prompt
	regexp.MustCompile(`(?i)Do you want to allow`),              // generic permission prompt
	regexp.MustCompile(`(?i)●\s*1\.\s*Allow\s+(once|for this)`), // gemini numbered option
}

// needsToolApproval checks if the screen shows an interactive tool permission prompt.
func needsToolApproval(screen string) bool {
	screen = stripANSI(screen)
	for _, p := range toolApprovalPatterns {
		if p.MatchString(screen) {
			return true
		}
	}
	return false
}

// providerWorkingPatterns matches progress indicators showing the provider is still active.
var providerWorkingPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Generating`),
	regexp.MustCompile(`(?i)Working\s*\(`),
	regexp.MustCompile(`(?i)Thinking`),
	regexp.MustCompile(`(?i)thinking with`),
	regexp.MustCompile(`(?i)Running\s+\w`),
	regexp.MustCompile(`(?i)Executing`),
	regexp.MustCompile(`(?i)Explored\b`),
	regexp.MustCompile(`(?i)✳`),                        // claude thinking indicator
	regexp.MustCompile(`[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]`),                 // braille spinner (gemini "taking a bit longer")
	regexp.MustCompile(`(?i)taking a bit longer`),      // gemini processing message
	regexp.MustCompile(`(?i)still on it`),              // gemini processing message
	regexp.MustCompile(`(?i)esc to cancel,\s*\d+[ms]`), // gemini cancel hint with elapsed time
}

// isProviderWorking checks if the screen shows progress indicators meaning the provider is active.
func isProviderWorking(screen string) bool {
	screen = stripANSI(screen)
	for _, p := range providerWorkingPatterns {
		if p.MatchString(screen) {
			return true
		}
	}
	return false
}

// isProviderStillWorking checks per-provider working patterns on screen.
// Returns true if any pattern matches, meaning the provider is still generating
// and completion should be deferred even if the idle prompt is visible.
// Returns false if patterns is nil/empty (provider has no working patterns).
func isProviderStillWorking(screen string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	cleaned := stripANSI(screen)
	for _, pat := range patterns {
		if strings.Contains(cleaned, pat) {
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

// CleanScreenForCrossPollination applies full sanitization for Round 2 cross-pollination.
// Strips TUI noise and self-assigned ICE scores (to prevent confidence cascade),
// but preserves all idea content, SCAMPER analysis, HMW questions, and reasoning.
func CleanScreenForCrossPollination(raw string) string {
	cleaned := cleanScreenOutput(raw)
	cleaned = stripICEScores(cleaned)
	return strings.TrimSpace(cleaned)
}

// stripICEScores removes self-assigned ICE scoring sections from provider output.
// This prevents confidence cascade where later rounds blindly adopt earlier scores.
func stripICEScores(s string) string {
	lines := strings.Split(s, "\n")
	filtered := make([]string, 0, len(lines))
	inICETable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Detect ICE table headers and skip until next non-table line
		if iceTableHeaderRe.MatchString(trimmed) {
			inICETable = true
			continue
		}
		if inICETable {
			// Stay in ICE table while lines look like table rows
			if strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "+-") || strings.HasPrefix(trimmed, "┌") || strings.HasPrefix(trimmed, "├") || strings.HasPrefix(trimmed, "└") || strings.HasPrefix(trimmed, "│") || trimmed == "" {
				continue
			}
			inICETable = false
		}
		// Skip standalone ICE score lines
		if iceScoreLineRe.MatchString(trimmed) {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}
