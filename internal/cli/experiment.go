package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/experiment"
)

// experimentFlags holds flags shared across experiment subcommands.
type experimentFlags struct {
	metric              string
	direction           string
	target              []string
	scope               []string
	maxIterations       int
	timeout             time.Duration
	metricKey           string
	metricRuns          int
	simplicityThreshold float64
	sessionID           string
}

// newExperimentCmd creates the `auto experiment` command group.
func newExperimentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experiment",
		Short: "Run and manage experiment loops",
	}

	cmd.AddCommand(newExperimentInitCmd())
	cmd.AddCommand(newExperimentMetricCmd())
	cmd.AddCommand(newExperimentRecordCmd())
	cmd.AddCommand(newExperimentCommitCmd())
	cmd.AddCommand(newExperimentResetCmd())
	cmd.AddCommand(newExperimentSummaryCmd())
	cmd.AddCommand(newExperimentStatusCmd())

	return cmd
}

// newExperimentInitCmd creates `auto experiment init`.
func newExperimentInitCmd() *cobra.Command {
	var f experimentFlags

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize an experiment branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}

			g := experiment.NewGit(dir)
			if err := g.CheckCleanWorktree(); err != nil {
				return fmt.Errorf("worktree must be clean before starting experiment: %w", err)
			}

			sessionID := f.sessionID
			if sessionID == "" {
				sessionID = fmt.Sprintf("%d", time.Now().UnixNano())
			}

			if err := g.CreateExperimentBranch(sessionID); err != nil {
				return fmt.Errorf("create experiment branch: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "experiment branch created: experiment/XLOOP-%s\n", sessionID)
			return nil
		},
	}

	addExperimentFlags(cmd, &f)
	return cmd
}

// newExperimentMetricCmd creates `auto experiment metric`.
func newExperimentMetricCmd() *cobra.Command {
	var f experimentFlags

	cmd := &cobra.Command{
		Use:   "metric",
		Short: "Run metric command and print result",
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.metric == "" {
				return fmt.Errorf("--metric is required")
			}

			cfg := buildConfig(f)
			out, err := experiment.RunMetricWithTimeout(cfg, f.metric)
			if err != nil {
				return fmt.Errorf("metric run failed: %w", err)
			}

			val, err := experiment.ExtractMetric(out, f.metricKey)
			if err != nil {
				return fmt.Errorf("extract metric: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "metric=%v unit=%s\n", val, out.Unit)
			return nil
		},
	}

	addExperimentFlags(cmd, &f)
	return cmd
}

// newExperimentRecordCmd creates `auto experiment record`.
func newExperimentRecordCmd() *cobra.Command {
	var (
		iteration   int
		status      string
		metricValue float64
		description string
	)

	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record an iteration result",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := experiment.Result{
				Iteration:   iteration,
				MetricValue: metricValue,
				Status:      status,
				Description: description,
				Timestamp:   time.Now(),
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(r)
		},
	}

	cmd.Flags().IntVar(&iteration, "iteration", 0, "Iteration number")
	cmd.Flags().StringVar(&status, "status", "keep", "Status: keep, discard, crash, timeout, scope-violation")
	cmd.Flags().Float64Var(&metricValue, "metric-value", 0, "Metric value for this iteration")
	cmd.Flags().StringVar(&description, "description", "", "Description of this iteration")

	return cmd
}

// newExperimentCommitCmd creates `auto experiment commit`.
func newExperimentCommitCmd() *cobra.Command {
	var (
		iteration   int
		description string
	)

	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Stage all changes and commit as experiment iteration",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}

			g := experiment.NewGit(dir)
			hash, err := g.CommitExperiment(iteration, description)
			if err != nil {
				return fmt.Errorf("commit experiment: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "committed: %s\n", hash)
			return nil
		},
	}

	cmd.Flags().IntVar(&iteration, "iteration", 0, "Iteration number")
	cmd.Flags().StringVar(&description, "description", "", "Description of this iteration")
	_ = cmd.MarkFlagRequired("iteration")

	return cmd
}

// newExperimentResetCmd creates `auto experiment reset`.
func newExperimentResetCmd() *cobra.Command {
	var commitHash string

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset working tree to a specific commit",
		RunE: func(cmd *cobra.Command, args []string) error {
			if commitHash == "" {
				return fmt.Errorf("--commit is required")
			}

			dir, err := os.Getwd()
			if err != nil {
				return err
			}

			g := experiment.NewGit(dir)
			if err := g.ResetToCommit(commitHash); err != nil {
				return fmt.Errorf("reset: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "reset to %s\n", commitHash)
			return nil
		},
	}

	cmd.Flags().StringVar(&commitHash, "commit", "", "Commit hash to reset to")
	return cmd
}
