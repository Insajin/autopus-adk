// Package spec provides SPEC path resolution across monorepo submodules.
package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveResult holds the resolved SPEC path information.
type ResolveResult struct {
	SpecDir      string // Full path to the SPEC directory
	SpecPath     string // Full path to spec.md
	TargetModule string // Submodule path, or "." for top-level
}

// ResolveSpecDir finds a SPEC directory by ID, searching top-level and submodules.
//
// Search order:
//  1. {baseDir}/.autopus/specs/{specID}/spec.md  (top-level)
//  2. {baseDir}/*/.autopus/specs/{specID}/spec.md (submodule depth 1)
//
// Returns an error if zero or multiple matches are found.
func ResolveSpecDir(baseDir, specID string) (*ResolveResult, error) {
	var matches []ResolveResult

	// Search 1: top-level
	topDir := filepath.Join(baseDir, ".autopus", "specs", specID)
	topSpec := filepath.Join(topDir, "spec.md")
	if _, err := os.Stat(topSpec); err == nil {
		matches = append(matches, ResolveResult{
			SpecDir:      topDir,
			SpecPath:     topSpec,
			TargetModule: ".",
		})
	}

	// Search 2: submodule depth 1
	entries, err := os.ReadDir(baseDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			// Skip hidden directories and common non-module dirs
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				continue
			}
			subDir := filepath.Join(baseDir, name, ".autopus", "specs", specID)
			subSpec := filepath.Join(subDir, "spec.md")
			if _, err := os.Stat(subSpec); err == nil {
				matches = append(matches, ResolveResult{
					SpecDir:      subDir,
					SpecPath:     subSpec,
					TargetModule: name,
				})
			}
		}
	}

	switch len(matches) {
	case 0:
		available := listAvailableSpecs(baseDir)
		if len(available) > 0 {
			return nil, fmt.Errorf("%s not found. Available SPECs: %s", specID, strings.Join(available, ", "))
		}
		return nil, fmt.Errorf("%s not found", specID)
	case 1:
		return &matches[0], nil
	default:
		var paths []string
		for _, m := range matches {
			paths = append(paths, m.SpecDir)
		}
		return nil, fmt.Errorf("duplicate %s found: %s", specID, strings.Join(paths, ", "))
	}
}

// listAvailableSpecs scans top-level and submodule SPEC directories.
func listAvailableSpecs(baseDir string) []string {
	var ids []string

	// Top-level
	ids = append(ids, scanSpecIDs(filepath.Join(baseDir, ".autopus", "specs"))...)

	// Submodules depth 1
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return ids
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		subIDs := scanSpecIDs(filepath.Join(baseDir, entry.Name(), ".autopus", "specs"))
		for _, id := range subIDs {
			ids = append(ids, fmt.Sprintf("%s (%s)", id, entry.Name()))
		}
	}

	return ids
}

// scanSpecIDs reads SPEC-* directories from a specs directory.
func scanSpecIDs(specsDir string) []string {
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil
	}

	var ids []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "SPEC-") {
			ids = append(ids, e.Name())
		}
	}
	return ids
}
