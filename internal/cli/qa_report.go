package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	qareport "github.com/insajin/autopus-adk/pkg/qa/report"
)

type qaReportOptions struct {
	ProjectDir       string
	RunIndexPath     string
	ReleaseIndexPath string
	OutputPath       string
	Title            string
	NoWrite          bool
	EmbedMedia       bool
	JSONOut          bool
	Format           string
}

type qaReportPayload struct {
	qareport.Report
	ReportPath string `json:"report_path,omitempty"`
}

func newQAReportCmd() *cobra.Command {
	var opts qaReportOptions
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Render a self-contained HTML report from QAMESH run and release evidence",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQAReport(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	cmd.Flags().StringVar(&opts.RunIndexPath, "run-index", "", "QAMESH run index path; defaults to latest")
	cmd.Flags().StringVar(&opts.ReleaseIndexPath, "release-index", "", "QAMESH release index path; defaults to latest")
	cmd.Flags().StringVar(&opts.OutputPath, "output", "", "Report output path; defaults to report.html beside the run index")
	cmd.Flags().StringVar(&opts.Title, "title", "", "Report title; defaults to the run id")
	cmd.Flags().BoolVar(&opts.NoWrite, "no-write", false, "Project the report without writing the HTML file")
	cmd.Flags().BoolVar(&opts.EmbedMedia, "embed-media", false,
		"Inline raw local screenshots from the capture directory; marks the report local-only and unshareable")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func runQAReport(cmd *cobra.Command, opts qaReportOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	errData := map[string]any{"project_dir": opts.ProjectDir}
	if strings.TrimSpace(opts.OutputPath) != "" {
		if err := rejectGeneratedQAOutput("output", opts.OutputPath); err != nil {
			return qaCommandError(cmd, jsonMode, err, "qa_report_output_rejected", errData)
		}
	}
	payload, err := buildQAReportPayload(opts)
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_report_failed", errData)
	}
	if jsonMode {
		return writeJSONResult(cmd, qaReportEnvelopeStatus(payload.Report), payload, nil, nil)
	}
	writeQAReportText(cmd, payload)
	return nil
}

func buildQAReportPayload(opts qaReportOptions) (qaReportPayload, error) {
	report, err := qareport.Build(qareport.Options{
		ProjectDir:       opts.ProjectDir,
		RunIndexPath:     opts.RunIndexPath,
		ReleaseIndexPath: opts.ReleaseIndexPath,
		Title:            opts.Title,
		EmbedMedia:       opts.EmbedMedia,
	})
	if err != nil {
		return qaReportPayload{}, err
	}
	if opts.NoWrite {
		return qaReportPayload{Report: report}, nil
	}
	output := strings.TrimSpace(opts.OutputPath)
	if output == "" {
		output = qareport.DefaultOutputPath(qaReportRunIndexPath(opts, report))
	}
	if err := qareport.WriteFile(report, output); err != nil {
		return qaReportPayload{}, fmt.Errorf("write report: %w", err)
	}
	return qaReportPayload{Report: report, ReportPath: output}, nil
}

// qaReportRunIndexPath prefers the explicit flag so the report lands beside the
// index the caller named; otherwise it reuses the resolved latest index.
func qaReportRunIndexPath(opts qaReportOptions, report qareport.Report) string {
	if strings.TrimSpace(opts.RunIndexPath) != "" {
		return opts.RunIndexPath
	}
	resolved, err := qareport.ResolveLatestIndex(opts.ProjectDir, qareport.RunsRelDir, qareport.RunIndexFile)
	if err != nil || resolved == "" {
		return report.Ingestion.RunIndexRef
	}
	return resolved
}

func qaReportEnvelopeStatus(report qareport.Report) jsonEnvelopeStatus {
	if report.Verdict == qareport.VerdictPassed {
		return jsonStatusOK
	}
	return jsonStatusWarn
}

// qaReportNextCommand is the hint printed right after a run produces evidence.
// That is the moment the visual report is renderable and the moment a human is
// most likely to want it, so discoverability lives in the run output rather than
// only in the docs.
func qaReportNextCommand(projectDir string) string {
	command := "auto qa report"
	if dir := strings.TrimSpace(projectDir); dir != "" && dir != "." {
		command += " --project-dir " + dir
	}
	return command
}

// writeQARunSummary prints the run status line and, when a run index exists, the
// report hint. Both the success and the failure path use it: a failed run has
// already written its index by the time Execute returns an error, and that is
// exactly when a human wants the visual report.
func writeQARunSummary(cmd *cobra.Command, status, runIndexPath, lane, projectDir string) {
	out := cmd.OutOrStdout()
	if lane != "" {
		fmt.Fprintf(out, "%s %s lane=%s\n", status, runIndexPath, lane)
	} else {
		fmt.Fprintf(out, "%s %s\n", status, runIndexPath)
	}
	if runIndexPath != "" {
		fmt.Fprintf(out, "next: %s\n", qaReportNextCommand(projectDir))
	}
}

func writeQAReportText(cmd *cobra.Command, payload qaReportPayload) {
	out := cmd.OutOrStdout()
	summary := payload.Summary
	fmt.Fprintf(out, "qa report %s ingestion=%s journeys=%d passed=%d failed=%d checks=%d failed_checks=%d artifacts=%d setup_gaps=%d retention=%s\n",
		payload.Verdict, payload.Ingestion.Status, summary.JourneyCount, summary.JourneysPassed,
		summary.JourneysFailed, summary.CheckCount, summary.ChecksFailed, summary.ArtifactCount,
		summary.SetupGapCount, payload.Retention)
	for _, rejection := range payload.Ingestion.Rejections {
		fmt.Fprintf(out, "rejected: %s (%s)\n", rejection.Ref, rejection.Reason)
	}
	if payload.ReportPath != "" {
		fmt.Fprintf(out, "report: %s\n", payload.ReportPath)
	}
	for _, next := range payload.NextCommands {
		fmt.Fprintf(out, "next: %s\n", next)
	}
}
