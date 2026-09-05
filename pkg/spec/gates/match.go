package gates

import (
	"path"
	"strings"
)

// MatchPattern reports whether the slash-separated relative path matches
// pattern with shell semantics: `**` matches zero or more path segments,
// every other segment is matched with path.Match, so `*.go` only matches
// top-level files while `**/*.go` matches at any depth.
func MatchPattern(pattern, rel string) bool {
	pattern = normalizeRel(pattern)
	rel = normalizeRel(rel)
	if pattern == "" || rel == "" {
		return false
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(rel, "/"))
}

func matchSegments(pattern, segments []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			if len(pattern) == 1 {
				return true
			}
			for i := 0; i <= len(segments); i++ {
				if matchSegments(pattern[1:], segments[i:]) {
					return true
				}
			}
			return false
		}
		if len(segments) == 0 {
			return false
		}
		if ok, err := path.Match(pattern[0], segments[0]); err != nil || !ok {
			return false
		}
		pattern, segments = pattern[1:], segments[1:]
	}
	return len(segments) == 0
}

// hasMeta reports whether s contains glob meta characters.
func hasMeta(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// literalPrefix returns the leading slash-joined segments of pattern that
// contain no meta characters; the remainder is the first meta segment onward.
func literalPrefix(pattern string) (prefix, rest string) {
	segments := strings.Split(pattern, "/")
	for i, segment := range segments {
		if hasMeta(segment) {
			return strings.Join(segments[:i], "/"), strings.Join(segments[i:], "/")
		}
	}
	return pattern, ""
}

// normalizeRel cleans a slash path; "." and "" both normalise to "".
func normalizeRel(p string) string {
	p = strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
	if p == "" {
		return ""
	}
	if p = path.Clean(p); p == "." {
		return ""
	}
	return p
}
