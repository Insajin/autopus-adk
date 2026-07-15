package memindex

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
)

var specIDRe = regexp.MustCompile(`SPEC-[A-Z0-9-]+-\d+|SPEC-[A-Z0-9-]+`)

// @AX:WARN [AUTO]: redaction boundary - every persisted or rendered memory text value must pass through safeText.
// @AX:REASON: Search/context output can include project, learning, and QAMESH text; bypassing this sanitizer risks secret leakage.
func safeText(value string) string {
	text := qaevidence.RedactText(value)
	text = strings.ReplaceAll(text, "\x00", " ")
	return compact(text, 1200)
}

func receiptSafeText(value string) string {
	text := qaevidence.RedactText(value)
	text = strings.ReplaceAll(text, "\x00", " ")
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}

func compact(value string, limit int) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if limit > 0 && len(value) > limit {
		return strings.TrimSpace(value[:limit]) + "..."
	}
	return value
}

func titleFromMarkdown(path string, body []byte) string {
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return filepath.Base(path)
}

func summaryFromMarkdown(body []byte) string {
	lines := make([]string, 0)
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "|") {
			continue
		}
		lines = append(lines, line)
		if len(strings.Join(lines, " ")) > 500 {
			break
		}
	}
	return safeText(strings.Join(lines, " "))
}

func detectSpecID(path, body string) string {
	if match := specIDRe.FindString(path); match != "" {
		return match
	}
	return specIDRe.FindString(body)
}

func tagsFromPath(rel string) []string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.TrimSuffix(part, filepath.Ext(part)))
		if part != "" && part != ".autopus" {
			tags = append(tags, strings.ToLower(part))
		}
	}
	sort.Strings(tags)
	return uniqueStrings(tags)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func sourceKindFromPath(rel string) string {
	rel = filepath.ToSlash(rel)
	switch {
	case strings.HasPrefix(rel, ".autopus/project/reviews/"):
		return "review_failure"
	case strings.HasPrefix(rel, ".autopus/project/"):
		return "project_doc"
	case strings.HasPrefix(rel, ".autopus/specs/"):
		return "spec"
	default:
		return "document"
	}
}

func ftsQuery(query string) string {
	terms := make([]string, 0)
	var b strings.Builder
	flush := func() {
		term := strings.TrimSpace(b.String())
		b.Reset()
		if term != "" {
			terms = append(terms, `"`+strings.ReplaceAll(term, `"`, `""`)+`"`)
		}
	}
	for _, r := range query {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	if len(terms) == 0 {
		return ""
	}
	return strings.Join(uniqueStrings(terms), " OR ")
}
