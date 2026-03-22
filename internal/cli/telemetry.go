package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/cost"
	"github.com/insajin/autopus-adk/pkg/telemetry"
)

// newTelemetryCmd creates the `auto telemetry` command group with subcommands
// for recording, summarising, and comparing pipeline run telemetry.
func newTelemetryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Manage pipeline telemetry",
	}

	cmd.AddCommand(newTelemetryRecordCmd())
	cmd.AddCommand(newTelemetrySummaryCmd())
	cmd.AddCommand(newTelemetryCostCmd())
	cmd.AddCommand(newTelemetryCompareCmd())

	return cmd
}

// newTelemetryRecordCmd creates `auto telemetry record` — an internal command
// used by agents to record pipeline, phase, and agent-run telemetry events.
func newTelemetryRecordCmd() *cobra.Command {
	var (
		specID      string
		agent       string
		phase       string
		action      string
		status      string
		files       int
		tokens      int
		qualityMode string
	)

	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record a telemetry event (internal agent use)",
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("telemetry record: get cwd: %w", err)
			}
			return runTelemetryRecord(baseDir, recordParams{
				specID:      specID,
				agent:       agent,
				phase:       phase,
				action:      action,
				status:      status,
				files:       files,
				tokens:      tokens,
				qualityMode: qualityMode,
			})
		},
	}

	cmd.Flags().StringVar(&specID, "spec-id", "", "SPEC identifier")
	cmd.Flags().StringVar(&agent, "agent", "", "Agent name")
	cmd.Flags().StringVar(&phase, "phase", "", "Phase name")
	cmd.Flags().StringVar(&action, "action", "", "Action: start | agent | end")
	cmd.Flags().StringVar(&status, "status", "PASS", "Status: PASS or FAIL")
	cmd.Flags().IntVar(&files, "files", 0, "Number of files modified")
	cmd.Flags().IntVar(&tokens, "tokens", 0, "Estimated token count")
	cmd.Flags().StringVar(&qualityMode, "quality-mode", "balanced", "Quality mode (ultra|balanced)")

	return cmd
}

// newTelemetrySummaryCmd creates `auto telemetry summary` — prints the most
// recent (or spec-filtered) pipeline run summary.
func newTelemetrySummaryCmd() *cobra.Command {
	var specID string

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Show latest pipeline summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("telemetry summary: get cwd: %w", err)
			}

			run, err := resolveSingleRun(baseDir, specID)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), telemetry.FormatSummary(*run))
			return nil
		},
	}

	cmd.Flags().StringVar(&specID, "spec-id", "", "Filter by SPEC identifier")
	return cmd
}

// newTelemetryCostCmd creates `auto telemetry cost` — prints a cost report for
// the most recent (or spec-filtered) pipeline run.
func newTelemetryCostCmd() *cobra.Command {
	var specID string

	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Show cost report for latest pipeline run",
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("telemetry cost: get cwd: %w", err)
			}

			run, err := resolveSingleRun(baseDir, specID)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), cost.FormatCostReport(*run))
			return nil
		},
	}

	cmd.Flags().StringVar(&specID, "spec-id", "", "Filter by SPEC identifier")
	return cmd
}

// newTelemetryCompareCmd creates `auto telemetry compare` — prints a side-by-side
// comparison of the two most recent pipeline runs (or filtered by --spec-id).
func newTelemetryCompareCmd() *cobra.Command {
	var specID string

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare two pipeline runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("telemetry compare: get cwd: %w", err)
			}

			runs, err := resolveTwoRuns(baseDir, specID)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), telemetry.FormatComparison(runs[0], runs[1]))
			return nil
		},
	}

	cmd.Flags().StringVar(&specID, "spec-id", "", "Filter runs by SPEC identifier")
	return cmd
}

