package release

import (
	"regexp"
	"strings"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
)

var (
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-004: redaction regexes define the release command-preview safety policy.
	sensitiveFlagRe = regexp.MustCompile(`(?i)^(--?(token|secret|password|pass|key|credential)|--?[A-Za-z0-9_.-]*(token|secret|password|pass|key|credential)[A-Za-z0-9_.-]*)$`)
	sensitiveEnvRe  = regexp.MustCompile(`(?i)^[A-Z0-9_.-]*(TOKEN|SECRET|PASSWORD|PASS|KEY|CREDENTIAL)[A-Z0-9_.-]*=`)
	credentialURLRe = regexp.MustCompile(`(?i)(https?://)[^/\s:@]+:[^/\s@]+@`)
	privatePathRe   = regexp.MustCompile(`(?i)(file://)?/(Users|home)/[^"'\n\r,)]*`)
	secretQueryRe   = regexp.MustCompile(`(?i)([?&][^=\s&]*(TOKEN|SECRET|PASSWORD|PASS|KEY|CREDENTIAL)[^=\s&]*=)[^&\s"']+`)
	runSensitiveRe  = regexp.MustCompile(`(?i)(--?[A-Za-z0-9_.-]*(token|secret|password|pass|key|credential)[A-Za-z0-9_.-]*(=|\s)|[A-Z0-9_.-]*(TOKEN|SECRET|PASSWORD|PASS|KEY|CREDENTIAL)[A-Z0-9_.-]*=|https?://[^/\s:@]+:[^/\s@]+@|/(Users|home)/)`)
)

func commandPreview(argv []string, run string) (string, bool) {
	if len(argv) == 0 && strings.TrimSpace(run) != "" && runSensitiveRe.MatchString(run) {
		return "<redacted>", true
	}
	parts := append([]string{}, argv...)
	if len(parts) == 0 && strings.TrimSpace(run) != "" {
		parts = strings.Fields(run)
	}
	redacted := false
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if key, _, ok := strings.Cut(part, "="); ok && sensitiveFlagRe.MatchString(key) {
			parts[i] = key + "=<redacted>"
			redacted = true
			continue
		}
		if sensitiveFlagRe.MatchString(part) && i+1 < len(parts) {
			parts[i+1] = "<redacted>"
			redacted = true
			continue
		}
		if sensitiveEnvRe.MatchString(part) {
			key, _, _ := strings.Cut(part, "=")
			parts[i] = key + "=<redacted>"
			redacted = true
			continue
		}
		masked, changed := redactURLsAndPaths(part)
		if changed {
			parts[i] = masked
			redacted = true
		}
	}
	return strings.Join(parts, " "), redacted
}

func redactURLsAndPaths(value string) (string, bool) {
	changed := false
	text := value
	nextURL := credentialURLRe.ReplaceAllString(text, "${1}<redacted>@")
	if nextURL != text {
		text = nextURL
		changed = true
	}
	nextQuery := secretQueryRe.ReplaceAllString(text, "${1}<redacted>")
	if nextQuery != text {
		text = nextQuery
		changed = true
	}
	next := privatePathRe.ReplaceAllString(text, "$PROJECT_ROOT")
	if next != text {
		changed = true
		text = next
	}
	return text, changed
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-004: release redaction bridges QAMESH evidence sentinels into the public release index placeholder format.
func redactReleaseString(value string) (string, bool) {
	if value == "" {
		return "", false
	}
	text := qaevidence.RedactText(value)
	text, changedURLs := redactURLsAndPaths(text)
	changed := changedURLs || text != value
	if strings.Contains(text, qaevidence.RedactedSecret) {
		text = strings.ReplaceAll(text, qaevidence.RedactedSecret, "<redacted>")
		changed = true
	}
	if strings.Contains(text, qaevidence.RedactedUser) {
		text = strings.ReplaceAll(text, "/Users/"+qaevidence.RedactedUser, "$PROJECT_ROOT")
		text = strings.ReplaceAll(text, "/home/"+qaevidence.RedactedUser, "$PROJECT_ROOT")
		changed = true
	}
	return text, changed
}

func mergeRedaction(states ...RedactionState) RedactionState {
	result := RedactionClean
	for _, state := range states {
		switch state {
		case RedactionBlocked:
			return RedactionBlocked
		case RedactionRedacted:
			result = RedactionRedacted
		}
	}
	return result
}
