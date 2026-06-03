package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/setup"
)

func setupValidateProjectContextFallback(cmd *cobra.Command, dir, docsDir string, jsonMode bool) (bool, error) {
	if pathExists(docsDir) {
		return false, nil
	}
	context := setup.DetectProjectContext(dir)
	if !context.Exists {
		return false, nil
	}

	report := setup.ValidateProjectContext(dir)
	if jsonMode {
		payload := buildSetupValidationPayload(dir, docsDir, report)
		warnings := buildSetupValidationWarnings(report)
		if report.Valid && len(report.Warnings) == 0 {
			return true, writeJSONResult(cmd, jsonStatusOK, payload, nil, nil)
		}
		return true, writeJSONResultAndExit(
			cmd,
			jsonStatusWarn,
			fmt.Errorf("%d project context issue(s) found", len(report.Warnings)),
			"setup_project_context_issues",
			payload,
			warnings,
			nil,
		)
	}

	out := cmd.OutOrStdout()
	if report.Valid && len(report.Warnings) == 0 {
		tui.Success(out, "Project context documents are present in .autopus/project/.")
		return true, nil
	}
	tui.Warnf(out, "Project context issues (%d):", len(report.Warnings))
	for _, w := range report.Warnings {
		tui.Bullet(out, fmt.Sprintf("[%s] %s: %s", w.Type, w.File, w.Message))
	}
	return true, fmt.Errorf("%d project context issue(s) found", len(report.Warnings))
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeProjectContextStatus(cmd *cobra.Command, status *setup.Status) {
	w := cmd.OutOrStdout()
	tui.SectionHeader(w, "Project Context Status")
	fmt.Fprintf(w, "Context dir: %s\n", status.ProjectContext.Dir)
	fmt.Fprintf(w, "Files:       %d\n\n", len(status.ProjectContext.Files))
	for _, fileName := range status.ProjectContext.Files {
		tui.OK(w, fileName)
	}
	if len(status.ProjectContext.MissingFiles) > 0 {
		fmt.Fprintln(w)
		tui.SectionHeader(w, "Missing Required Files")
		for _, fileName := range status.ProjectContext.MissingFiles {
			tui.FAIL(w, fileName)
		}
	}
	fmt.Fprintln(w)
	tui.Info(w, ".autopus/docs bundle is not generated; .autopus/project is the canonical workspace context.")
}
