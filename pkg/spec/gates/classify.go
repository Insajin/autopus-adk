package gates

import (
	"path"
	"sort"
	"strings"
)

// Classification is the deterministic breakdown of a change set.
type Classification struct {
	Class         ChangeClass
	Paths         []string // normalised, sorted, deduplicated
	UIPaths       []string // paths that touch a UI surface
	SecurityPaths []string // paths that touch a security or data surface
	Roots         []string // distinct module roots spanned by non-doc paths
}

// HasUI reports whether the change set touches a UI surface.
func (c Classification) HasUI() bool { return len(c.UIPaths) > 0 }

var uiExtensions = map[string]bool{
	".tsx": true, ".jsx": true, ".vue": true, ".svelte": true,
	".css": true, ".scss": true, ".less": true,
}

var uiDirSegments = map[string]bool{
	"components": true, "pages": true, "app": true, "ui": true,
}

var uiSegmentPrefixes = []string{"theme", "design-system", "tokens"}

var securityTokens = map[string]bool{
	"auth": true, "acl": true, "permission": true, "rbac": true, "policy": true,
	"migration": true, "migrations": true, "schema": true, "sql": true, "db": true,
	"database": true, "crypto": true, "secret": true, "token": true,
}

var docExtensions = map[string]bool{
	".md": true, ".mdx": true, ".markdown": true, ".txt": true, ".rst": true, ".adoc": true,
}

// rootedPrefixes are first segments that are too coarse to be a module root;
// their first two segments form the root instead.
var rootedPrefixes = map[string]bool{
	"pkg": true, "internal": true, "cmd": true, "src": true, "app": true,
}

// ClassifyChange returns the change class of paths; uiGlobs are additional
// configured UI globs (config design.ui_globs).
func ClassifyChange(paths []string, uiGlobs []string) ChangeClass {
	return Classify(paths, uiGlobs).Class
}

// Classify breaks a change set down deterministically. Precedence:
// doc_only > security_or_data > multi_domain > ui_only > general.
// An empty change set is classified general so that every gate stays required.
func Classify(paths []string, uiGlobs []string) Classification {
	result := Classification{Paths: normalizePaths(paths)}
	if len(result.Paths) == 0 {
		result.Class = ClassGeneral
		return result
	}
	roots := map[string]bool{}
	docs := 0
	for _, p := range result.Paths {
		if IsDocPath(p) {
			docs++
			continue
		}
		if IsUIPath(p, uiGlobs) {
			result.UIPaths = append(result.UIPaths, p)
		}
		if IsSecurityPath(p) {
			result.SecurityPaths = append(result.SecurityPaths, p)
		}
		if root := moduleRoot(p); root != "" {
			roots[root] = true
		}
	}
	for root := range roots {
		result.Roots = append(result.Roots, root)
	}
	sort.Strings(result.Roots)

	codePaths := len(result.Paths) - docs
	switch {
	case codePaths == 0:
		result.Class = ClassDocOnly
	case len(result.SecurityPaths) > 0:
		result.Class = ClassSecurityOrData
	case len(result.Roots) >= 2:
		result.Class = ClassMultiDomain
	case len(result.UIPaths) == codePaths:
		result.Class = ClassUIOnly
	default:
		result.Class = ClassGeneral
	}
	return result
}

// IsDocPath reports whether p is documentation.
func IsDocPath(p string) bool {
	p = normalizeRel(p)
	if docExtensions[strings.ToLower(path.Ext(p))] {
		return true
	}
	first, _, _ := strings.Cut(p, "/")
	return strings.EqualFold(first, "docs") || strings.EqualFold(first, "doc")
}

// IsUIPath reports whether p touches a UI surface by extension, directory
// hint, or a configured glob. Configured globs without a slash also match
// the base name at any depth, as the design context detector does.
func IsUIPath(p string, uiGlobs []string) bool {
	p = normalizeRel(p)
	lower := strings.ToLower(p)
	if uiExtensions[path.Ext(lower)] {
		return true
	}
	segments := strings.Split(lower, "/")
	for _, segment := range segments[:len(segments)-1] {
		if uiDirSegments[segment] {
			return true
		}
	}
	for _, segment := range segments {
		for _, prefix := range uiSegmentPrefixes {
			if strings.HasPrefix(segment, prefix) {
				return true
			}
		}
	}
	for _, glob := range uiGlobs {
		glob = normalizeRel(glob)
		if MatchPattern(glob, p) {
			return true
		}
		if !strings.Contains(glob, "/") {
			if ok, _ := path.Match(glob, path.Base(p)); ok {
				return true
			}
		}
	}
	return false
}

// IsSecurityPath reports whether any path segment token names a security or
// data surface. Segments are split on '-', '_' and '.' so that
// auth_service.go and 001.sql both match while tokens.css does not.
func IsSecurityPath(p string) bool {
	for _, segment := range strings.Split(strings.ToLower(normalizeRel(p)), "/") {
		for _, token := range strings.FieldsFunc(segment, isTokenSeparator) {
			if securityTokens[token] {
				return true
			}
		}
	}
	return false
}

func isTokenSeparator(r rune) bool { return r == '-' || r == '_' || r == '.' }

// moduleRoot returns the module root of p: the first segment, or the first
// two when the first is a generic container (pkg, internal, cmd, src, app).
// Top-level files have no root.
func moduleRoot(p string) string {
	segments := strings.Split(normalizeRel(p), "/")
	if len(segments) < 2 {
		return ""
	}
	if rootedPrefixes[segments[0]] && len(segments) > 2 {
		return segments[0] + "/" + segments[1]
	}
	return segments[0]
}

// normalizePaths cleans, deduplicates, and sorts paths, dropping empties.
func normalizePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		rel := normalizeRel(p)
		if rel == "" || seen[rel] {
			continue
		}
		seen[rel] = true
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}
