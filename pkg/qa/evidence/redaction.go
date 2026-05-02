package evidence

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	RedactedSecret      = "[REDACTED_SECRET]"
	RedactedUser        = "[REDACTED_USER]"
	RedactedPrivateNote = "[REDACTED_PRIVATE_NOTE]"
)

type Finding struct {
	Type   string `json:"type"`
	Source string `json:"source"`
	Sample string `json:"sample"`
}

var (
	secretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\bBearer\s+[A-Za-z0-9._~+/=-]{12,}\b`),
		regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9_-]{16,}\b`),
		regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{16,}\b`),
		regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{20,}\b`),
		regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`),
	}
	sensitiveAssignmentRe = regexp.MustCompile(`(?i)\b([A-Z0-9_.-]*(TOKEN|SECRET|PASSWORD|PASSWD|PWD|API[_-]?KEY|PRIVATE[_-]?KEY|ACCESS[_-]?KEY|CREDENTIAL|COOKIE|SESSION|AUTH)[A-Z0-9_.-]*)(\s*[:=]\s*)(["']?)([^\s"',}\]]{3,})(["']?)`)
	jsonSensitiveRe       = regexp.MustCompile(`(?i)("[^"]*(token|secret|password|passwd|pwd|api[_-]?key|apikey|private[_-]?key|access[_-]?key|credential|cookie|session|authorization|auth)[^"]*"\s*:\s*)("[^"]*"|[^",\n}\]]+)`)
	privateNoteRe         = regexp.MustCompile(`(?im)\b((local[_ -]?vault[_ -]?note|vault[_ -]?note|private[_ -]?note|private_note_body|note[_ -]?body|localNoteBody|vaultNoteBody|vaultNoteContent)[^:=\n"{}]*\s*[:=]\s*)([^,\n\r}]*)`)
	jsonPrivateNoteRe     = regexp.MustCompile(`(?i)("[^"]*(localNote|vaultNote|privateNote|noteBody|note_body|noteContent|note_content)[^"]*"\s*:\s*)("[^"]*"|[^",\n}\]]+)`)
	userPathRe            = regexp.MustCompile(`(file://)?/(Users|home)/([^/\s:"']+)(/[^\s"',)]*)?`)
	windowsUserPathRe     = regexp.MustCompile(`([A-Za-z]:\\+Users\\+)([^\\\s:"']+)((?:\\+[^\s"',)]*)?)`)
)

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-QAMESH-001: redaction API protects publishable QA evidence from provider tokens, private notes, and local user paths.
// @AX:REASON: Manifest writing, artifact sanitization, and feedback generation depend on this function preserving safe placeholders before publication.
func RedactText(value string) string {
	if value == "" {
		return ""
	}
	text := value
	for _, re := range secretPatterns {
		text = re.ReplaceAllStringFunc(text, func(match string) string {
			if strings.HasPrefix(match, "Bearer ") {
				return "Bearer " + RedactedSecret
			}
			return RedactedSecret
		})
	}
	text = sensitiveAssignmentRe.ReplaceAllString(text, `${1}${3}${4}`+RedactedSecret+`${6}`)
	text = jsonSensitiveRe.ReplaceAllString(text, `${1}"`+RedactedSecret+`"`)
	text = privateNoteRe.ReplaceAllString(text, `${1}`+RedactedPrivateNote)
	text = jsonPrivateNoteRe.ReplaceAllString(text, `${1}"`+RedactedPrivateNote+`"`)
	text = userPathRe.ReplaceAllStringFunc(text, func(match string) string {
		parts := userPathRe.FindStringSubmatch(match)
		if len(parts) < 5 || parts[3] == RedactedUser {
			return match
		}
		return fmt.Sprintf("%s/%s/%s%s", parts[1], parts[2], RedactedUser, parts[4])
	})
	text = windowsUserPathRe.ReplaceAllString(text, `${1}`+RedactedUser+`${3}`)
	return text
}

func FindUnsafeText(value, source string) []Finding {
	text := value
	findings := make([]Finding, 0)
	for _, re := range secretPatterns {
		for _, match := range re.FindAllString(text, -1) {
			if !strings.Contains(match, "[REDACTED") {
				findings = append(findings, Finding{Type: "secret", Source: source, Sample: RedactText(compactSample(match))})
			}
		}
	}
	for _, match := range sensitiveAssignmentRe.FindAllStringSubmatch(text, -1) {
		if len(match) > 5 && !strings.Contains(match[5], "[REDACTED") {
			findings = append(findings, Finding{Type: "sensitive_assignment", Source: source, Sample: RedactText(compactSample(match[0]))})
		}
	}
	for _, match := range jsonSensitiveRe.FindAllStringSubmatch(text, -1) {
		if len(match) > 3 && !strings.Contains(match[3], "[REDACTED") {
			findings = append(findings, Finding{Type: "sensitive_json_value", Source: source, Sample: RedactText(compactSample(match[0]))})
		}
	}
	for _, match := range privateNoteRe.FindAllStringSubmatch(text, -1) {
		if len(match) > 3 && strings.TrimSpace(match[3]) != "" && !strings.Contains(match[3], "[REDACTED") {
			findings = append(findings, Finding{Type: "private_note", Source: source, Sample: RedactText(compactSample(match[0]))})
		}
	}
	for _, match := range jsonPrivateNoteRe.FindAllStringSubmatch(text, -1) {
		if len(match) > 3 && strings.TrimSpace(match[3]) != "" && !strings.Contains(match[3], "[REDACTED") {
			findings = append(findings, Finding{Type: "private_note_json_value", Source: source, Sample: RedactText(compactSample(match[0]))})
		}
	}
	for _, match := range userPathRe.FindAllStringSubmatch(text, -1) {
		if len(match) > 3 && match[3] != RedactedUser {
			findings = append(findings, Finding{Type: "local_user_path", Source: source, Sample: RedactText(compactSample(match[0]))})
		}
	}
	for _, match := range windowsUserPathRe.FindAllStringSubmatch(text, -1) {
		if len(match) > 2 && match[2] != RedactedUser {
			findings = append(findings, Finding{Type: "local_user_path", Source: source, Sample: RedactText(compactSample(match[0]))})
		}
	}
	return findings
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-QAMESH-001: unsafe text assertion is the final gate before writing publishable evidence or repair prompts.
// @AX:REASON: Multiple writers call this guard to fail closed when redaction misses secrets, private notes, or local user paths.
func AssertSafeText(value, source string) error {
	findings := FindUnsafeText(value, source)
	if len(findings) == 0 {
		return nil
	}
	return fmt.Errorf("unsafe evidence in %s: %s", source, FormatFindings(findings))
}

func FormatFindings(findings []Finding) string {
	lines := make([]string, 0, len(findings))
	for _, finding := range findings {
		lines = append(lines, fmt.Sprintf("%s:%s", finding.Type, RedactText(finding.Source)))
	}
	return strings.Join(lines, "; ")
}

func compactSample(value string) string {
	compacted := strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(value, " "))
	if len(compacted) > 120 {
		return compacted[:120]
	}
	return compacted
}
