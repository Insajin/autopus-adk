package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/setup"
)

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Generate and manage project documentation for AI agents",
	}

	cmd.AddCommand(newSetupGenerateCmd())
	cmd.AddCommand(newSetupUpdateCmd())
	cmd.AddCommand(newSetupValidateCmd())
	cmd.AddCommand(newSetupStatusCmd())
	return cmd
}

func newSetupGenerateCmd() *cobra.Command {
	var (
		force     bool
		outputDir string
		preview   bool
	)

	cmd := &cobra.Command{
		Use:   "generate [dir]",
		Short: "Generate project documentation in .autopus/docs/",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := resolveDirFromArgs(args)
			if err != nil {
				return err
			}

			opts := &setup.GenerateOptions{
				OutputDir: outputDir,
				Force:     force,
			}

			if preview {
				plan, planErr := setup.BuildGeneratePlan(dir, opts)
				if planErr != nil {
					return planErr
				}
				printPreview(cmd.OutOrStdout(), "auto setup generate", setupPreviewHint(plan), setupPreviewItems(plan))
				return nil
			}

			_, genErr := setup.Generate(dir, opts)
			if genErr != nil {
				return genErr
			}

			tui.Success(cmd.OutOrStdout(), "Documentation generated in .autopus/docs/")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing documentation")
	cmd.Flags().StringVar(&outputDir, "output", "", "Output directory (default: .autopus/docs/)")
	cmd.Flags().BoolVar(&preview, "plan", false, "변경 예정 문서만 계산하고 쓰지 않음")
	cmd.Flags().BoolVar(&preview, "preview", false, "변경 예정 문서만 계산하고 쓰지 않음")
	cmd.Flags().BoolVar(&preview, "dry-run", false, "변경 예정 문서만 계산하고 쓰지 않음")
	return cmd
}

func newSetupUpdateCmd() *cobra.Command {
	var outputDir string
	var preview bool

	cmd := &cobra.Command{
		Use:   "update [dir]",
		Short: "Update changed documentation files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := resolveDirFromArgs(args)
			if err != nil {
				return err
			}

			if preview {
				plan, planErr := setup.BuildUpdatePlan(dir, outputDir)
				if planErr != nil {
					return planErr
				}
				printPreview(cmd.OutOrStdout(), "auto setup update", setupPreviewHint(plan), setupPreviewItems(plan))
				return nil
			}

			updated, updateErr := setup.Update(dir, outputDir)
			if updateErr != nil {
				return updateErr
			}

			out := cmd.OutOrStdout()
			if len(updated) == 0 {
				tui.Info(out, "All documents are up to date.")
			} else {
				tui.Successf(out, "Updated %d document(s):", len(updated))
				for _, f := range updated {
					tui.Bullet(out, f)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "output", "", "Documentation directory (default: .autopus/docs/)")
	cmd.Flags().BoolVar(&preview, "plan", false, "변경 예정 문서만 계산하고 쓰지 않음")
	cmd.Flags().BoolVar(&preview, "preview", false, "변경 예정 문서만 계산하고 쓰지 않음")
	cmd.Flags().BoolVar(&preview, "dry-run", false, "변경 예정 문서만 계산하고 쓰지 않음")
	return cmd
}

func newSetupValidateCmd() *cobra.Command {
	var outputDir, format string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "validate [dir]",
		Short: "Validate documentation against current code state",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := resolveDirFromArgs(args)
			if err != nil {
				return err
			}

			jsonMode, err := resolveJSONMode(jsonOutput, format)
			if err != nil {
				return err
			}

			docsDir := resolveOutputDir(dir, outputDir)
			report, valErr := setup.Validate(docsDir, dir)
			if valErr != nil {
				if jsonMode {
					return writeJSONResultAndExit(
						cmd,
						jsonStatusError,
						valErr,
						"setup_validate_failed",
						map[string]any{"project_dir": dir, "docs_dir": docsDir},
						nil,
						nil,
					)
				}
				return valErr
			}

			// Also check command validity
			cmdWarnings := setup.ValidateCommands(docsDir, dir)
			report.Warnings = append(report.Warnings, cmdWarnings...)
			if len(cmdWarnings) > 0 {
				report.Valid = false
			}

			if jsonMode {
				payload := buildSetupValidationPayload(dir, docsDir, report)
				warnings := buildSetupValidationWarnings(report)
				if report.Valid && len(report.Warnings) == 0 {
					return writeJSONResult(cmd, jsonStatusOK, payload, nil, nil)
				}
				return writeJSONResultAndExit(
					cmd,
					jsonStatusWarn,
					fmt.Errorf("%d validation issue(s) found", len(report.Warnings)),
					"setup_validation_issues",
					payload,
					warnings,
					nil,
				)
			}

			out := cmd.OutOrStdout()
			if report.Valid && len(report.Warnings) == 0 {
				tui.Success(out, "All documents are up to date.")
				return nil
			}

			tui.Warnf(out, "Validation issues (%d):", len(report.Warnings))
			for _, w := range report.Warnings {
				loc := w.File
				if w.Line > 0 {
					loc = fmt.Sprintf("%s:%d", w.File, w.Line)
				}
				tui.Bullet(out, fmt.Sprintf("[%s] %s: %s", w.Type, loc, w.Message))
			}
			fmt.Fprintf(out, "\n  Drift score: %.1f%%\n", report.DriftScore*100)

			return fmt.Errorf("%d validation issue(s) found", len(report.Warnings))
		},
	}

	cmd.Flags().StringVar(&outputDir, "output", "", "Documentation directory (default: .autopus/docs/)")
	addJSONFlags(cmd, &jsonOutput, &format)
	return cmd
}

func newSetupStatusCmd() *cobra.Command {
	var outputDir, format string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status [dir]",
		Short: "Show documentation status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := resolveDirFromArgs(args)
			if err != nil {
				return err
			}

			jsonMode, err := resolveJSONMode(jsonOutput, format)
			if err != nil {
				return err
			}

			status, statusErr := setup.GetStatus(dir, outputDir)
			if statusErr != nil {
				if jsonMode {
					return writeJSONResultAndExit(
						cmd,
						jsonStatusError,
						statusErr,
						"setup_status_failed",
						map[string]any{"project_dir": dir},
						nil,
						nil,
					)
				}
				return statusErr
			}

			if jsonMode {
				warnings := buildSetupStatusWarnings(status)
				statusValue := jsonStatusOK
				if len(warnings) > 0 {
					statusValue = jsonStatusWarn
				}
				return writeJSONResult(cmd, statusValue, buildSetupStatusPayload(dir, outputDir, status), warnings, nil)
			}

			if !status.Exists {
				tui.Info(cmd.OutOrStdout(), "No documentation found. Run `auto setup generate` to create.")
				return nil
			}

			w := cmd.OutOrStdout()
			tui.SectionHeader(w, "Documentation Status")

			if !status.GeneratedAt.IsZero() {
				ago := time.Since(status.GeneratedAt).Round(time.Hour)
				fmt.Fprintf(w, "Last generated: %s (%s ago)\n",
					status.GeneratedAt.Format("2006-01-02 15:04"),
					ago)
			}
			fmt.Fprintf(w, "Drift score:    %.1f%%\n\n", status.DriftScore*100)

			tui.SectionHeader(w, "Files")
			for fileName, fs := range status.FileStatuses {
				if !fs.Exists {
					tui.FAIL(w, fmt.Sprintf("%-20s missing", fileName))
				} else if fs.Fresh {
					tui.OK(w, fmt.Sprintf("%-20s fresh", fileName))
				} else {
					tui.SKIP(w, fmt.Sprintf("%-20s stale", fileName))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "output", "", "Documentation directory (default: .autopus/docs/)")
	addJSONFlags(cmd, &jsonOutput, &format)
	return cmd
}

func resolveDirFromArgs(args []string) (string, error) {
	if len(args) > 0 { //nolint:revive
		return resolveDir(args[0])
	}
	return resolveDir("")
}

func resolveOutputDir(projectDir, outputDir string) string {
	if outputDir != "" { //nolint:revive
		return outputDir
	}
	return projectDir + "/.autopus/docs"
}
