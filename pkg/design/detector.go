package design

import (
	"path/filepath"
	"strings"
)

// @AX:NOTE [AUTO]: UI file heuristics gate design-context injection for review and verify workflows.
var uiExts = map[string]bool{
	".tsx": true, ".jsx": true, ".css": true, ".scss": true, ".sass": true, ".less": true,
}

func IsUIRelatedFile(path string, configuredGlobs []string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	if uiExts[strings.ToLower(filepath.Ext(clean))] {
		return true
	}
	lower := strings.ToLower(clean)
	for _, segment := range strings.Split(lower, "/") {
		if strings.Contains(segment, "token") || strings.Contains(segment, "theme") || strings.Contains(segment, "design-system") {
			return true
		}
	}
	for _, glob := range configuredGlobs {
		if matchesGlob(clean, glob) {
			return true
		}
	}
	return false
}

func AnyUIRelatedFile(paths []string, configuredGlobs []string) bool {
	for _, path := range paths {
		if IsUIRelatedFile(path, configuredGlobs) {
			return true
		}
	}
	return false
}

func matchesGlob(path, glob string) bool {
	glob = filepath.ToSlash(strings.TrimSpace(glob))
	if glob == "" {
		return false
	}
	if ok, _ := filepath.Match(glob, path); ok {
		return true
	}
	if ok, _ := filepath.Match(glob, filepath.Base(path)); ok {
		return true
	}
	return strings.HasPrefix(path, strings.TrimSuffix(glob, "*"))
}
