// Package e2e provides user-facing scenario-based E2E test infrastructure.
package e2e

import (
	"path/filepath"
	"regexp"
	"strings"
)

// BuildEntry represents a single build command with optional label and submodule mapping.
type BuildEntry struct {
	Command       string // e.g., "go build ./cmd/auto/"
	Label         string // e.g., "ADK" (empty for single build)
	SubmodulePath string // e.g., "autopus-adk" (resolved from label)
}

// @AX:NOTE [AUTO] @AX:REASON: magic constant — label-to-submodule mapping; must stay in sync with sectionLabelMap and scenarios.md build line format
// defaultSubmoduleMap maps known labels to their submodule directory paths.
var defaultSubmoduleMap = map[string]string{
	"ADK":      "autopus-adk",
	"Backend":  "Autopus/backend",
	"Frontend": "Autopus/frontend",
}

// reLabelSuffix matches a trailing "(Label)" at the end of a build command.
var reLabelSuffix = regexp.MustCompile(`\s*\(([^)]+)\)\s*$`)

// ParseBuildLine parses a comma-separated build line into individual BuildEntries.
// Each entry may have an optional (Label) suffix that maps to a submodule path.
func ParseBuildLine(line string) []BuildEntry {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil
	}

	parts := strings.Split(trimmed, ",")
	entries := make([]BuildEntry, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		entry := BuildEntry{}
		if m := reLabelSuffix.FindStringSubmatchIndex(part); m != nil {
			entry.Label = part[m[2]:m[3]]
			entry.Command = strings.TrimSpace(part[:m[0]])
			entry.SubmodulePath = defaultSubmoduleMap[entry.Label]
		} else {
			entry.Command = part
		}

		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return nil
	}
	return entries
}

// ResolveBuildDir resolves a BuildEntry's SubmodulePath to a full directory path.
// Falls back to projectDir when SubmodulePath is empty.
func ResolveBuildDir(projectDir string, entry BuildEntry) string {
	if entry.SubmodulePath == "" {
		return projectDir
	}
	return filepath.Join(projectDir, entry.SubmodulePath)
}

// @AX:NOTE [AUTO] @AX:REASON: magic constant — section-to-label mapping; must stay in sync with defaultSubmoduleMap
// sectionLabelMap maps section header keywords to build labels.
var sectionLabelMap = map[string]string{
	"ADK":      "ADK",
	"Backend":  "Backend",
	"Frontend": "Frontend",
}

// MatchBuild finds the BuildEntry matching a scenario's section header.
// Returns nil if no match is found or builds is empty.
// A single unlabeled build matches any scenario.
func MatchBuild(scenario Scenario, builds []BuildEntry) *BuildEntry {
	if len(builds) == 0 || scenario.Section == "" {
		return nil
	}

	// Single unlabeled build matches everything.
	if len(builds) == 1 && builds[0].Label == "" {
		b := builds[0]
		return &b
	}

	// Extract label from section header by checking known keywords.
	for keyword, label := range sectionLabelMap {
		if strings.Contains(scenario.Section, keyword) {
			for i := range builds {
				if builds[i].Label == label {
					b := builds[i]
					return &b
				}
			}
		}
	}

	return nil
}
