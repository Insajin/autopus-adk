package orchestra

import (
	"regexp"
	"strings"
	"time"
)

const claudeReadyPromptPattern = `(?m)^[\t ]*❯[ \t\x{00a0}]*(?:Try\b[^\r\n]*)?[ \t\x{00a0}]*$`

var codexReadyBlockerPattern = regexp.MustCompile(
	`(?im)^\s*(?:PostCompact hooks|Turn hooks on or off.*|Press esc to go back)\s*$`,
)

// SessionReadyPatterns returns completion patterns for CLI session readiness detection.
// Unlike DefaultCompletionPatterns, this excludes shell prompts ($ and #) to prevent
// false positives when detecting whether a CLI tool has finished launching.
func SessionReadyPatterns() []CompletionPattern {
	return []CompletionPattern{
		{Provider: "claude", Pattern: regexp.MustCompile(claudeReadyPromptPattern)},
		{Provider: "codex", Pattern: regexp.MustCompile(codexReadyPromptPattern)},
		{Provider: "gemini", Pattern: regexp.MustCompile(`(?m)^\s*>\s*(Type your|@|\s*$)`)},
		{Provider: "opencode", Pattern: regexp.MustCompile(`(?im)^Ask anything\s*$`)},
	}
}

// isSessionReady checks if the screen content contains a CLI-specific prompt pattern,
// indicating the provider session has fully launched. Unlike isPromptVisible, this does
// NOT match shell prompts ($ and #) to avoid false positives during startup.
func isSessionReady(screen string, patterns []CompletionPattern) bool {
	if len(patterns) == 0 {
		patterns = SessionReadyPatterns()
	}
	screen = stripANSI(screen)
	for _, cp := range patterns {
		if isSessionReadyBlocked(screen, cp.Provider) {
			continue
		}
		if cp.Pattern != nil && cp.Pattern.MatchString(screen) {
			return true
		}
	}
	return false
}

// isProviderSessionReady matches only the requested provider's input prompt.
// This prevents one pane's startup chrome from satisfying another provider's
// gate when multiple providers launch concurrently.
func isProviderSessionReady(screen string, patterns []CompletionPattern, provider string) bool {
	screen = stripANSI(screen)
	if isSessionReadyBlocked(screen, provider) {
		return false
	}
	for _, cp := range patterns {
		if !strings.EqualFold(strings.TrimSpace(cp.Provider), strings.TrimSpace(provider)) {
			continue
		}
		if cp.Pattern != nil && cp.Pattern.MatchString(screen) {
			return true
		}
	}
	return false
}

func isSessionReadyBlocked(screen, provider string) bool {
	return strings.EqualFold(strings.TrimSpace(provider), "codex") && codexReadyBlockerPattern.MatchString(screen)
}

// startupTimeoutFor returns the per-provider startup timeout.
func startupTimeoutFor(provider ProviderConfig) time.Duration {
	if provider.StartupTimeout > 0 {
		return provider.StartupTimeout
	}
	switch provider.Name {
	case "claude":
		return 15 * time.Second
	case "gemini":
		return 10 * time.Second
	default:
		return 30 * time.Second
	}
}
