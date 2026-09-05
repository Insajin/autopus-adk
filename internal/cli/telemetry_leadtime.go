package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

// leadTimeParams holds the parsed flags for `auto telemetry leadtime`.
type leadTimeParams struct {
	runID    string
	baseline string
}

// telemetryLeadTimePayload is the body-free JSON data for `auto telemetry
// leadtime`. Baseline is present only when --baseline was given.
type telemetryLeadTimePayload struct {
	SpecID      string                        `json:"spec_id"`
	StartTime   time.Time                     `json:"start_time"`
	FinalStatus string                        `json:"final_status"`
	LeadTime    telemetry.LeadTimeReport      `json:"leadtime"`
	Baseline    *telemetry.LeadTimeComparison `json:"baseline,omitempty"`
}

// newTelemetryLeadTimeCmd creates `auto telemetry leadtime` — reports lead
// time, critical path, rework, defects, and safety-gate omissions for a run,
// optionally against a baseline run. A baseline regression (more escaped
// defects or more unresolved safety gates) exits non-zero.
func newTelemetryLeadTimeCmd() *cobra.Command {
	var p leadTimeParams
	var jsonOutput bool
	var format string

	cmd := &cobra.Command{
		Use:   "leadtime",
		Short: "Show lead-time, critical-path, and defect report for a pipeline run",
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("telemetry leadtime: get cwd: %w", err)
			}
			jsonMode, err := resolveJSONMode(jsonOutput, format)
			if err != nil {
				return err
			}
			return runTelemetryLeadTime(cmd, baseDir, p, jsonMode)
		},
	}

	cmd.Flags().StringVar(&p.runID, "run", "", "SPEC identifier of the run to report (latest run of that SPEC; default: latest run)")
	cmd.Flags().StringVar(&p.baseline, "baseline", "", "Baseline run: a SPEC identifier (its previous run when it is the reported SPEC) or a project directory")
	addJSONFlags(cmd, &jsonOutput, &format)
	return cmd
}

// runTelemetryLeadTime resolves the run (and baseline), computes the report,
// and writes it. It is extracted from RunE for testability.
func runTelemetryLeadTime(cmd *cobra.Command, baseDir string, p leadTimeParams, jsonMode bool) error {
	run, err := resolveSingleRun(baseDir, p.runID)
	if err != nil {
		return leadTimeFailure(cmd, jsonMode, err, "telemetry_leadtime_unavailable", p)
	}
	payload := telemetryLeadTimePayload{
		SpecID:      run.SpecID,
		StartTime:   run.StartTime,
		FinalStatus: run.FinalStatus,
		LeadTime:    telemetry.ComputeLeadTime(*run),
	}
	warnings := leadTimeWarnings(payload.LeadTime)
	if p.baseline != "" {
		base, err := resolveBaselineRun(baseDir, p.baseline, p.runID, *run)
		if err != nil {
			return leadTimeFailure(cmd, jsonMode, err, "telemetry_leadtime_baseline_unavailable", p)
		}
		comparison := telemetry.CompareLeadTime(payload.LeadTime, telemetry.ComputeLeadTime(*base))
		payload.Baseline = &comparison
	}

	var regression error
	if payload.Baseline != nil && payload.Baseline.Regression {
		regression = fmt.Errorf("telemetry leadtime: regression versus baseline %s: %s",
			payload.Baseline.BaselineSpecID, strings.Join(payload.Baseline.RegressionReasons, "; "))
	}
	if jsonMode {
		if regression != nil {
			return writeJSONResultAndExit(cmd, jsonStatusError, regression, "leadtime_regression", payload, warnings, nil)
		}
		status := jsonStatusOK
		if len(warnings) > 0 {
			status = jsonStatusWarn
		}
		return writeJSONResult(cmd, status, payload, warnings, nil)
	}
	_, _ = fmt.Fprint(cmd.OutOrStdout(), telemetry.FormatLeadTime(payload.LeadTime, payload.Baseline))
	return regression
}

func leadTimeFailure(cmd *cobra.Command, jsonMode bool, cause error, code string, p leadTimeParams) error {
	if !jsonMode {
		return cause
	}
	return writeJSONResultAndExit(cmd, jsonStatusError, cause, code,
		map[string]any{"run": p.runID, "baseline": p.baseline}, nil, nil)
}

func leadTimeWarnings(report telemetry.LeadTimeReport) []jsonMessage {
	warnings := make([]jsonMessage, 0, len(report.Issues))
	for _, issue := range report.Issues {
		warnings = append(warnings, jsonMessage{Code: "leadtime_issue", Message: issue})
	}
	return warnings
}

// resolveBaselineRun picks the baseline: an existing directory is another
// project (its latest run, filtered by --run when given); otherwise the value
// is a SPEC identifier — its latest run, or its previous run when it is the
// SPEC being reported so a run is never compared against itself.
func resolveBaselineRun(baseDir, baseline, runID string, current telemetry.PipelineRun) (*telemetry.PipelineRun, error) {
	if info, err := os.Stat(baseline); err == nil && info.IsDir() {
		return resolveSingleRun(baseline, runID)
	}
	runs, err := telemetry.PipelineRunsBySpecID(baseDir, baseline)
	if err != nil {
		return nil, fmt.Errorf("telemetry: load baseline runs: %w", err)
	}
	if baseline != current.SpecID {
		if len(runs) == 0 {
			return nil, fmt.Errorf("telemetry: no baseline runs found for %q (not a SPEC identifier or directory)", baseline)
		}
		return &runs[len(runs)-1], nil
	}
	if len(runs) < 2 {
		return nil, fmt.Errorf("telemetry: no earlier run of %q to use as baseline", baseline)
	}
	return &runs[len(runs)-2], nil
}
