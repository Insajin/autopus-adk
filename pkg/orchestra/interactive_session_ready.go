package orchestra

import (
	"regexp"
	"time"
)

// sessionReadyPromptPatterns matches CLI-specific prompts WITHOUT shell patterns ($ and #).
// Used by waitForSessionReady to avoid premature detection on bare shell prompts.
var sessionReadyPromptPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^❯\s*$`),                     // claude code prompt (unicode heavy right-pointing angle)
	regexp.MustCompile(`(?m)^\s*>\s*(Type your|@|\s*$)`), // gemini TUI prompt (> Type your..., > @, bare >)
	regexp.MustCompile(`(?im)^codex>\s*$`),               // codex prompt (case-insensitive)
	regexp.MustCompile(`(?im)^Ask anything\s*$`),         // opencode TUI prompt
	// NOTE: no shell $ or # patterns — this is the key difference from defaultPromptPatterns
}

// SessionReadyPatterns returns completion patterns for CLI session readiness detection.
// Unlike DefaultCompletionPatterns, this excludes shell prompts ($ and #) to prevent
// false positives when detecting whether a CLI tool has finished launching.
func SessionReadyPatterns() []CompletionPattern {
	return []CompletionPattern{
		{Provider: "claude", Pattern: regexp.MustCompile(`(?m)^❯\s*$`)},
		{Provider: "codex", Pattern: regexp.MustCompile(`(?im)^codex>\s*$`)},
		{Provider: "gemini", Pattern: regexp.MustCompile(`(?m)^\s*>\s*(Type your|@|\s*$)`)},
		{Provider: "opencode", Pattern: regexp.MustCompile(`(?im)^Ask anything\s*$`)},
	}
}

// isSessionReady checks if the screen content contains a CLI-specific prompt pattern,
// indicating the provider session has fully launched. Unlike isPromptVisible, this does
// NOT match shell prompts ($ and #) to avoid false positives during startup.
func isSessionReady(screen string, patterns []CompletionPattern) bool {
	screen = stripANSI(screen)
	for _, cp := range patterns {
		if cp.Pattern.MatchString(screen) {
			return true
		}
	}
	for _, p := range sessionReadyPromptPatterns {
		if p.MatchString(screen) {
			return true
		}
	}
	return false
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
