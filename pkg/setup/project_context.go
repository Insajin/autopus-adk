package setup

import (
	"os"
	"path/filepath"
	"sort"
)

var requiredProjectContextFiles = []string{
	"product.md",
	"scenarios.md",
	"structure.md",
	"tech.md",
	"workspace.md",
}

// ProjectContextStatus reports the canonical .autopus/project documentation set.
type ProjectContextStatus struct {
	Exists       bool
	Dir          string
	Files        []string
	MissingFiles []string
}

// DetectProjectContext detects the canonical workspace context document set.
func DetectProjectContext(projectDir string) ProjectContextStatus {
	dir := filepath.Join(projectDir, ".autopus", "project")
	status := ProjectContextStatus{Dir: dir}
	entries, err := os.ReadDir(dir)
	if err != nil {
		status.MissingFiles = append(status.MissingFiles, requiredProjectContextFiles...)
		return status
	}

	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		status.Files = append(status.Files, entry.Name())
		seen[entry.Name()] = true
	}
	sort.Strings(status.Files)
	status.Exists = len(status.Files) > 0

	for _, required := range requiredProjectContextFiles {
		if !seen[required] {
			status.MissingFiles = append(status.MissingFiles, required)
		}
	}
	return status
}

// ValidateProjectContext validates the canonical .autopus/project context set.
func ValidateProjectContext(projectDir string) *ValidationReport {
	context := DetectProjectContext(projectDir)
	report := &ValidationReport{Valid: true}
	if !context.Exists {
		report.Valid = false
		report.Warnings = append(report.Warnings, ValidationWarning{
			File:    ".autopus/project",
			Message: "Project context directory missing: .autopus/project",
			Type:    "missing_project_context",
		})
		report.DriftScore = 1.0
		return report
	}
	for _, missing := range context.MissingFiles {
		report.Valid = false
		report.Warnings = append(report.Warnings, ValidationWarning{
			File:    filepath.ToSlash(filepath.Join(".autopus", "project", missing)),
			Message: "Project context file missing: " + missing,
			Type:    "missing_project_context",
		})
	}
	if len(report.Warnings) > 0 {
		report.DriftScore = float64(len(report.Warnings)) / float64(len(requiredProjectContextFiles))
		if report.DriftScore > 1.0 {
			report.DriftScore = 1.0
		}
	}
	return report
}
