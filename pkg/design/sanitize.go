package design

import (
	"regexp"
	"strings"
)

// @AX:NOTE [AUTO]: Sanitizer version, secret regexes, and injection phrases are prompt-safety policy constants.
const sanitizerVersion = "design-sanitizer-v1"

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`(?i)(api[_-]?key|token|secret)\s*[:=]\s*[A-Za-z0-9._-]{16,}`),
}

type sanitizeResult struct {
	Content    string
	Rejected   bool
	Reasons    []string
	Redactions int
}

func sanitizeImportContent(content string) sanitizeResult {
	lower := strings.ToLower(content)
	reasons := promptInjectionReasons(lower)
	if len(reasons) > 0 {
		return sanitizeResult{Rejected: true, Reasons: reasons}
	}
	redacted := content
	redactions := 0
	for _, pattern := range secretPatterns {
		matches := pattern.FindAllString(redacted, -1)
		redactions += len(matches)
		redacted = pattern.ReplaceAllString(redacted, "[REDACTED_SECRET]")
	}
	return sanitizeResult{Content: redacted, Redactions: redactions}
}

func promptInjectionReasons(lower string) []string {
	checks := map[string]string{
		"ignore previous":   "prompt_injection",
		"reveal the system": "prompt_exfiltration",
		"system prompt":     "prompt_exfiltration",
		"<script":           "script_surface",
		"javascript:":       "script_surface",
		"role: system":      "role_override",
		"you are now":       "role_override",
	}
	var reasons []string
	seen := map[string]bool{}
	for needle, reason := range checks {
		if strings.Contains(lower, needle) && !seen[reason] {
			reasons = append(reasons, reason)
			seen[reason] = true
		}
	}
	return reasons
}
