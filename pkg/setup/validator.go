package setup

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Matches paths in backticks: `path/to/file`
var backtickPathRe = regexp.MustCompile("`([a-zA-Z0-9_./-]+(?:/[a-zA-Z0-9_./-]+)+)`")

// Validate checks documentation against current project state.
func Validate(docsDir, projectDir string) (*ValidationReport, error) {
	report := &ValidationReport{Valid: true}

	// Check each doc file
	for name, fileName := range DocFiles {
		docPath := filepath.Join(docsDir, fileName)
		if _, err := os.Stat(docPath); err != nil {
			report.Warnings = append(report.Warnings, ValidationWarning{
				File:    fileName,
				Message: "Document file missing: " + fileName,
				Type:    "missing_doc",
			})
			report.Valid = false
			continue
		}

		validateDocFile(docPath, name, projectDir, report)
	}

	// Check meta
	if _, err := os.Stat(filepath.Join(docsDir, metaFileName)); err != nil {
		report.Warnings = append(report.Warnings, ValidationWarning{
			File:    metaFileName,
			Message: ".meta.yaml is missing",
			Type:    "missing_doc",
		})
		report.Valid = false
	}

	// Calculate drift score
	if len(report.Warnings) > 0 {
		report.DriftScore = float64(len(report.Warnings)) / float64(len(DocFiles)*3) // normalize
		if report.DriftScore > 1.0 {
			report.DriftScore = 1.0
		}
	}

	return report, nil
}

func validateDocFile(docPath, docName, projectDir string, report *ValidationReport) {
	f, err := os.Open(docPath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	lineCount := 0
	inCodeBlock := false

	for scanner.Scan() {
		lineNum++
		lineCount++
		line := scanner.Text()

		// Track code blocks
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inCodeBlock {
				inCodeBlock = false
			} else {
				inCodeBlock = true
				// Check language identifier
				trimmed := strings.TrimSpace(line)
				if trimmed == "```" && !inCodeBlock {
					// Opening code block without language ID
					report.Warnings = append(report.Warnings, ValidationWarning{
						File:    filepath.Base(docPath),
						Line:    lineNum,
						Message: "Code block without language identifier",
						Type:    "missing_lang_id",
					})
				}
			}
			continue
		}

		if inCodeBlock {
			continue
		}

		// Check path references
		matches := backtickPathRe.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			refPath := match[1]
			// Skip URLs and non-file references
			if strings.Contains(refPath, "://") || strings.HasPrefix(refPath, "sha256:") {
				continue
			}
			fullPath := filepath.Join(projectDir, refPath)
			if _, err := os.Stat(fullPath); err != nil {
				report.Warnings = append(report.Warnings, ValidationWarning{
					File:    filepath.Base(docPath),
					Line:    lineNum,
					Message: "Stale reference: " + refPath + " no longer exists",
					Type:    "stale_path",
				})
				report.Valid = false
			}
		}
	}

	// Check line limits
	maxLines := maxDocLines
	if docName == "index" {
		maxLines = maxIndexLines
	}
	if lineCount > maxLines {
		report.Warnings = append(report.Warnings, ValidationWarning{
			File:    filepath.Base(docPath),
			Message: "Exceeds line limit: " + filepath.Base(docPath),
			Type:    "line_limit",
		})
	}
}

// ValidateCommands checks that documented commands are still valid.
func ValidateCommands(docsDir, projectDir string) []ValidationWarning {
	var warnings []ValidationWarning

	cmdPath := filepath.Join(docsDir, "commands.md")
	if _, err := os.Stat(cmdPath); err != nil {
		return warnings
	}

	// Check that build files referenced in commands.md still exist
	f, err := os.Open(cmdPath)
	if err != nil {
		return warnings
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check for build file references
		matches := backtickPathRe.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			refPath := match[1]
			if isBuildFileName(refPath) {
				fullPath := filepath.Join(projectDir, refPath)
				if _, err := os.Stat(fullPath); err != nil {
					warnings = append(warnings, ValidationWarning{
						File:    "commands.md",
						Line:    lineNum,
						Message: "Referenced build file no longer exists: " + refPath,
						Type:    "stale_command",
					})
				}
			}
		}
	}
	return warnings
}

func isBuildFileName(name string) bool {
	buildFiles := map[string]bool{
		"Makefile":            true,
		"package.json":        true,
		"go.mod":              true,
		"Cargo.toml":          true,
		"pyproject.toml":      true,
		"docker-compose.yml":  true,
		"docker-compose.yaml": true,
		"compose.yml":         true,
		"compose.yaml":        true,
	}
	return buildFiles[filepath.Base(name)]
}
