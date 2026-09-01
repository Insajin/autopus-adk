package evidence

import (
	"regexp"
	"strings"
)

// A credential-shaped key also introduces the harness's own prose: the mobile
// readiness gap reads "missing_credentials: opaque credential refs are required
// before mobile execution". Redacting "opaque" corrupted a diagnostic while
// protecting nothing, so an assignment value survives only when it has no
// credential shape AND the line keeps reading as a sentence. Both conditions are
// required because a bare short lowercase value is indistinguishable from a weak
// password ("password: letmein"), and the redactor must stay fail-closed there.
const maxProseAssignmentValue = 16

// proseContinuationRe matches ` <lowercase word>` followed by sentence
// punctuation or end of line. A trailing `=` or `:` is excluded on purpose: that
// shape is the next key of a machine-readable line, not prose.
var proseContinuationRe = regexp.MustCompile(`^[ \t]+[a-z]{2,}(?:[ \t.,;)\]}"']|$)`)

// Submatch group numbers of the value-bearing patterns. They are addressed by
// index because rewriting from match offsets is what lets the prose gate read
// the text that follows a match.
const (
	assignmentQuoteGroup = 4
	assignmentValueGroup = 5
	flagValueGroup       = 4
)

func redactSensitiveAssignments(text string) string {
	return redactValueGroup(sensitiveAssignmentRe, text, assignmentValueGroup, isProseAssignment)
}

func redactSensitiveFlagValues(text string) string {
	return redactValueGroup(sensitiveFlagValueRe, text, flagValueGroup, nil)
}

// redactValueGroup replaces one submatch group of every match with the
// placeholder, leaving the rest of the match byte-identical.
//
// Already-redacted values are skipped: the placeholder's trailing `]` falls
// outside these value character classes, so a second pass used to append another
// bracket. Redaction runs repeatedly over the same bytes (publish, then feedback
// bundle export, then prompt rendering), so it must be idempotent.
func redactValueGroup(re *regexp.Regexp, text string, group int, exempt func(string, []int) bool) string {
	matches := re.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	last := 0
	for _, match := range matches {
		start, end := groupBounds(match, group)
		if start < 0 || strings.Contains(text[start:end], "[REDACTED") {
			continue
		}
		if exempt != nil && exempt(text, match) {
			continue
		}
		b.WriteString(text[last:start])
		b.WriteString(RedactedSecret)
		last = end
	}
	b.WriteString(text[last:])
	return b.String()
}

// isProseAssignment reports whether the assignment at match is harness prose:
// an unquoted short all-lowercase word whose line continues with another
// lowercase word. FindUnsafeText consults the same predicate, so the publish
// gate never reports "passed" over text the redactor declined to touch.
func isProseAssignment(text string, match []int) bool {
	if groupText(text, match, assignmentQuoteGroup) != "" {
		return false
	}
	value := groupText(text, match, assignmentValueGroup)
	if value == "" || len(value) > maxProseAssignmentValue || !isLowerAlphaWord(value) {
		return false
	}
	return proseContinuationRe.MatchString(text[match[1]:])
}

func assignmentValue(text string, match []int) string {
	return groupText(text, match, assignmentValueGroup)
}

func groupBounds(match []int, group int) (int, int) {
	if len(match) <= 2*group+1 {
		return -1, -1
	}
	return match[2*group], match[2*group+1]
}

func groupText(text string, match []int, group int) string {
	start, end := groupBounds(match, group)
	if start < 0 || end < start || end > len(text) {
		return ""
	}
	return text[start:end]
}

func isLowerAlphaWord(value string) bool {
	for index := range len(value) {
		if value[index] < 'a' || value[index] > 'z' {
			return false
		}
	}
	return value != ""
}
