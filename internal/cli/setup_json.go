package cli

import (
	"sort"
	"time"

	"github.com/insajin/autopus-adk/pkg/setup"
)

type setupStatusFilePayload struct {
	Name       string    `json:"name"`
	Exists     bool      `json:"exists"`
	Fresh      bool      `json:"fresh"`
	ModifiedAt time.Time `json:"modified_at,omitempty"`
}

type setupStatusPayload struct {
	ProjectDir  string                   `json:"project_dir"`
	DocsDir     string                   `json:"docs_dir"`
	Exists      bool                     `json:"exists"`
	GeneratedAt time.Time                `json:"generated_at,omitempty"`
	DriftScore  float64                  `json:"drift_score"`
	Files       []setupStatusFilePayload `json:"files"`
}

type setupValidationWarningPayload struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

type setupValidationPayload struct {
	ProjectDir   string                          `json:"project_dir"`
	DocsDir      string                          `json:"docs_dir"`
	Valid        bool                            `json:"valid"`
	DriftScore   float64                         `json:"drift_score"`
	WarningCount int                             `json:"warning_count"`
	Warnings     []setupValidationWarningPayload `json:"warnings"`
}

func buildSetupStatusPayload(projectDir, outputDir string, status *setup.Status) setupStatusPayload {
	files := make([]setupStatusFilePayload, 0, len(status.FileStatuses))
	names := make([]string, 0, len(status.FileStatuses))
	for name := range status.FileStatuses {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		fileStatus := status.FileStatuses[name]
		files = append(files, setupStatusFilePayload{
			Name:       name,
			Exists:     fileStatus.Exists,
			Fresh:      fileStatus.Fresh,
			ModifiedAt: fileStatus.ModTime,
		})
	}

	return setupStatusPayload{
		ProjectDir:  projectDir,
		DocsDir:     resolveOutputDir(projectDir, outputDir),
		Exists:      status.Exists,
		GeneratedAt: status.GeneratedAt,
		DriftScore:  status.DriftScore,
		Files:       files,
	}
}

func buildSetupStatusWarnings(status *setup.Status) []jsonMessage {
	if status.Exists {
		return nil
	}
	return []jsonMessage{{
		Code:    "docs_not_found",
		Message: "No documentation found. Run `auto setup generate` to create.",
	}}
}

func buildSetupValidationPayload(projectDir, docsDir string, report *setup.ValidationReport) setupValidationPayload {
	warnings := make([]setupValidationWarningPayload, 0, len(report.Warnings))
	for _, warning := range report.Warnings {
		warnings = append(warnings, setupValidationWarningPayload{
			File:    warning.File,
			Line:    warning.Line,
			Type:    warning.Type,
			Message: warning.Message,
		})
	}

	return setupValidationPayload{
		ProjectDir:   projectDir,
		DocsDir:      docsDir,
		Valid:        report.Valid,
		DriftScore:   report.DriftScore,
		WarningCount: len(report.Warnings),
		Warnings:     warnings,
	}
}

func buildSetupValidationWarnings(report *setup.ValidationReport) []jsonMessage {
	if len(report.Warnings) == 0 {
		return nil
	}

	warnings := make([]jsonMessage, 0, len(report.Warnings))
	for _, warning := range report.Warnings {
		warnings = append(warnings, jsonMessage{
			Code:    warning.Type,
			Message: warning.Message,
		})
	}
	return warnings
}
