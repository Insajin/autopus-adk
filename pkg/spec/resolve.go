// Package spec provides SPEC path resolution across monorepo submodules.
package spec

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// walkSkipDirs are directories excluded from recursive SPEC resolution.
var walkSkipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".cache":       true,
	"dist":         true,
}

// ResolveResult holds the resolved SPEC path information.
type ResolveResult struct {
	SpecDir      string // Full path to the SPEC directory
	SpecPath     string // Full path to spec.md
	TargetModule string // Submodule path, or "." for top-level
}

// ResolveSpecDir finds a SPEC directory by ID, searching top-level and all submodule depths.
//
// Search order:
//  1. {baseDir}/.autopus/specs/{specID}/spec.md  (top-level, fast path)
//  2. {baseDir}/**/.autopus/specs/{specID}/spec.md (recursive walk, any depth)
//
// Returns an error if zero or multiple matches are found.
func ResolveSpecDir(baseDir, specID string) (*ResolveResult, error) {
	var matches []ResolveResult

	// Search 1: top-level fast path
	topDir := filepath.Join(baseDir, ".autopus", "specs", specID)
	topSpec := filepath.Join(topDir, "spec.md")
	if _, err := os.Stat(topSpec); err == nil {
		matches = append(matches, ResolveResult{
			SpecDir:      topDir,
			SpecPath:     topSpec,
			TargetModule: ".",
		})
	}

	// Search 2: recursive walk to find .autopus/specs/{specID}/spec.md at any depth
	_ = filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable paths
		}
		if !d.IsDir() {
			return nil
		}
		// The walk root must always be entered. When baseDir is "." the
		// root's d.Name() is "." which would otherwise match the hidden
		// directory filter and abort the traversal (GitHub issue #36).
		if path == baseDir {
			return nil
		}
		name := d.Name()
		// Skip directories that should not be searched
		if walkSkipDirs[name] || (strings.HasPrefix(name, ".") && name != ".autopus") {
			return filepath.SkipDir
		}
		// Detect pattern: <anything>/.autopus/specs/{specID}
		if name != specID {
			return nil
		}
		parent := filepath.Dir(path)
		if filepath.Base(parent) != "specs" {
			return nil
		}
		grandparent := filepath.Dir(parent)
		if filepath.Base(grandparent) != ".autopus" {
			return nil
		}
		specMd := filepath.Join(path, "spec.md")
		if _, err := os.Stat(specMd); err != nil {
			return nil
		}
		// Determine target module relative to baseDir
		module, err := filepath.Rel(baseDir, filepath.Dir(grandparent))
		if err != nil || module == "." {
			// Already captured by top-level fast path
			return nil
		}
		matches = append(matches, ResolveResult{
			SpecDir:      path,
			SpecPath:     specMd,
			TargetModule: module,
		})
		return filepath.SkipDir
	})

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

// listAvailableSpecs recursively scans SPEC directories at any depth.
func listAvailableSpecs(baseDir string) []string {
	var ids []string

	_ = filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		// Same baseDir=="." fix as in ResolveSpecDir: never skip the walk root.
		if path == baseDir {
			return nil
		}
		name := d.Name()
		// Skip non-relevant directories
		if walkSkipDirs[name] || (strings.HasPrefix(name, ".") && name != ".autopus") {
			return filepath.SkipDir
		}
		// Detect .autopus/specs directories
		if name != "specs" {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) != ".autopus" {
			return nil
		}
		// Compute label: module path relative to baseDir
		module, err := filepath.Rel(baseDir, filepath.Dir(filepath.Dir(path)))
		if err != nil {
			module = "."
		}

		specEntries, err := os.ReadDir(path)
		if err != nil {
			return filepath.SkipDir
		}
		for _, e := range specEntries {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), "SPEC-") {
				continue
			}
			if module == "." {
				ids = append(ids, e.Name())
			} else {
				ids = append(ids, fmt.Sprintf("%s (%s)", e.Name(), module))
			}
		}
		return filepath.SkipDir
	})

	return ids
}
