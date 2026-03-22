package constraint

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// CheckOptions configures the violation check.
type CheckOptions struct {
	// Categories filters checks to specific categories. Empty means all categories.
	Categories []Category
	// Extensions filters files by extension (e.g., ".go", ".ts"). Empty means all extensions.
	Extensions []string
}

// @AX:NOTE [AUTO]: Public API entry point for violation scanning
// Check scans files in the given directory for constraint violations.
// It walks the directory recursively, reads each file line by line,
// and reports lines that contain a constraint pattern as a substring.
// Hidden directories, vendor, node_modules, and testdata are skipped.
func Check(dir string, constraints []Constraint, opts CheckOptions) ([]Violation, error) {
	if len(constraints) == 0 {
		return nil, nil
	}

	filtered := filterByCategory(constraints, opts.Categories)
	if len(filtered) == 0 {
		return nil, nil
	}

	var violations []Violation
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if info.IsDir() {
			base := filepath.Base(path)
			// @AX:NOTE [AUTO]: Skipped directory names — extend this list if new build artifact dirs are introduced
			if strings.HasPrefix(base, ".") || base == "vendor" || base == "node_modules" || base == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !matchesExtension(path, opts.Extensions) {
			return nil
		}

		fileViolations, scanErr := checkFile(path, filtered)
		if scanErr != nil {
			// @AX:TODO [AUTO]: Propagate or log scan errors — SPEC-ANTI-001 @AX:CYCLE:2
			return nil // skip unreadable files
		}
		violations = append(violations, fileViolations...)
		return nil
	})

	return violations, err
}

// checkFile scans a single file for constraint violations line by line.
func checkFile(path string, constraints []Constraint) ([]Violation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var violations []Violation
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for _, c := range constraints {
			if strings.Contains(line, c.Pattern) {
				violations = append(violations, Violation{
					Constraint: c,
					File:       path,
					Line:       lineNum,
					Match:      strings.TrimSpace(line),
				})
			}
		}
	}
	return violations, scanner.Err()
}

// FormatViolations produces a human-readable violation report.
func FormatViolations(violations []Violation) string {
	if len(violations) == 0 {
		return "No constraint violations found."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d constraint violation(s):\n\n", len(violations))
	for i, v := range violations {
		fmt.Fprintf(&sb, "%d. %s:%d\n", i+1, v.File, v.Line)
		fmt.Fprintf(&sb, "   Pattern: %s\n", v.Constraint.Pattern)
		fmt.Fprintf(&sb, "   Reason:  %s\n", v.Constraint.Reason)
		fmt.Fprintf(&sb, "   Suggest: %s\n", v.Constraint.Suggest)
		fmt.Fprintf(&sb, "   Match:   %s\n", v.Match)
	}
	return sb.String()
}

// filterByCategory returns constraints belonging to one of the given categories.
// When categories is empty, all constraints are returned unchanged.
func filterByCategory(constraints []Constraint, categories []Category) []Constraint {
	if len(categories) == 0 {
		return constraints
	}
	catSet := make(map[Category]bool, len(categories))
	for _, c := range categories {
		catSet[c] = true
	}
	var result []Constraint
	for _, c := range constraints {
		if catSet[c.Category] {
			result = append(result, c)
		}
	}
	return result
}

// matchesExtension reports whether path has one of the given file extensions.
// When extensions is empty, all files match.
func matchesExtension(path string, extensions []string) bool {
	if len(extensions) == 0 {
		return true
	}
	return slices.Contains(extensions, filepath.Ext(path))
}
