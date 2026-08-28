package orchestra

import (
	"regexp"
	"strings"
)

var (
	lineColumnSuffix = regexp.MustCompile(`(?i)(?::\d+(?::\d+)?(?:-\d+)?)$`)
	githubLineSuffix = regexp.MustCompile(`(?i)#L\d+(?:-L?\d+)?$`)
	wordLineSuffix   = regexp.MustCompile(`(?i)\s*\(?lines?\s+\d+(?:-\d+)?\)?$`)
)

func stableFindingIdentity(finding Finding) string {
	if id := normalizeLine(finding.ID); id != "" {
		return "id|" + id
	}
	scope := finding.ScopeRef
	if strings.TrimSpace(scope) == "" {
		scope = finding.Location
	}
	parts := []string{
		normalizeLine(finding.Category),
		normalizeStableScope(scope),
		normalizeLine(finding.Description),
	}
	return strings.Join(parts, "|")
}

func normalizeStableScope(scope string) string {
	scope = strings.TrimSpace(scope)
	scope = lineColumnSuffix.ReplaceAllString(scope, "")
	scope = githubLineSuffix.ReplaceAllString(scope, "")
	scope = wordLineSuffix.ReplaceAllString(scope, "")
	return normalizeLine(scope)
}
