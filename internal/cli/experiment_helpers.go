package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/experiment"
)

// newExperimentSummaryCmd creates `auto experiment summary`.
func newExperimentSummaryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Print experiment summary from stdin (JSON results)",
		RunE: func(cmd *cobra.Command, args []string) error {
			rec := experiment.NewRecorder()

			dec := json.NewDecoder(cmd.InOrStdin())
			for dec.More() {
				var r experiment.Result
				if err := dec.Decode(&r); err != nil {
					return fmt.Errorf("decode result: %w", err)
				}
				rec.Record(r)
			}

			s := rec.Summary()
			fmt.Fprintf(cmd.OutOrStdout(),
				"total=%d keep=%d discard=%d best=%.4f (iter %d)\n",
				s.TotalIterations, s.KeepCount, s.DiscardCount,
				s.BestMetric, s.BestIteration,
			)
			return nil
		},
	}
}

// newExperimentStatusCmd creates `auto experiment status`.
func newExperimentStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current experiment branch status",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}

			g := experiment.NewGit(dir)
			isExp, err := g.IsExperimentBranch()
			if err != nil {
				return fmt.Errorf("check branch: %w", err)
			}

			if !isExp {
				fmt.Fprintln(cmd.OutOrStdout(), "not on an experiment branch")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "on experiment branch")

			if err := g.CheckCleanWorktree(); err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "worktree: dirty")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "worktree: clean")
			}

			return nil
		},
	}
}

// buildConfig constructs an experiment.Config from CLI flags.
func buildConfig(f experimentFlags) experiment.Config {
	cfg := experiment.DefaultConfig()

	cfg.MetricCmd = f.metric
	cfg.MetricKey = f.metricKey
	cfg.TargetFiles = f.target
	cfg.Scope = f.scope
	cfg.MaxIterations = f.maxIterations
	cfg.ExperimentTimeout = f.timeout
	cfg.MetricRuns = f.metricRuns
	cfg.SimplicityThreshold = f.simplicityThreshold
	cfg.SessionID = f.sessionID

	if strings.ToLower(f.direction) == "maximize" {
		cfg.Direction = experiment.Maximize
	} else {
		cfg.Direction = experiment.Minimize
	}

	return cfg
}

// addExperimentFlags attaches common experiment flags to cmd.
func addExperimentFlags(cmd *cobra.Command, f *experimentFlags) {
	cmd.Flags().StringVar(&f.metric, "metric", "", "Shell command that outputs a JSON metric (required)")
	cmd.Flags().StringVar(&f.direction, "direction", "minimize", "Optimization direction: minimize|maximize")
	cmd.Flags().StringSliceVar(&f.target, "target", nil, "Target files for modification (required)")
	cmd.Flags().StringSliceVar(&f.scope, "scope", nil, "Allowed file scope (default: same as --target)")
	cmd.Flags().IntVar(&f.maxIterations, "max-iterations", 50, "Maximum number of iterations")
	cmd.Flags().DurationVar(&f.timeout, "timeout", 5*time.Minute, "Timeout per metric run")
	cmd.Flags().StringVar(&f.metricKey, "metric-key", "", "JSON key to extract from metric output")
	cmd.Flags().IntVar(&f.metricRuns, "metric-runs", 1, "Number of metric runs for median")
	cmd.Flags().Float64Var(&f.simplicityThreshold, "simplicity-threshold", 0.001, "Minimum simplicity score to keep")
	cmd.Flags().StringVar(&f.sessionID, "session-id", "", "Experiment session ID (auto-generated if empty)")
}
