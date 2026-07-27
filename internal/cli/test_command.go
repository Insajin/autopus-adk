package cli

import (
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/spf13/cobra"
)

// newAutoTestCmd creates the `auto test` parent command with the `run` subcommand.
func newAutoTestCmd() *cobra.Command {
	parent := &cobra.Command{
		Use:           "test",
		Short:         "Run E2E scenarios against the project",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	parent.AddCommand(newAutoTestRunCmd())
	return parent
}

// newAutoTestRunCmd creates the `auto test run` subcommand.
func newAutoTestRunCmd() *cobra.Command {
	var (
		scenarioID string
		jsonOut    bool
		format     string
		profile    string
		timeout    time.Duration
		verbose    bool
		projectDir string
		validation string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute E2E scenarios and report PASS/FAIL per scenario",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAutoTest(cmd, scenarioID, jsonOut, format, profile, timeout, verbose, projectDir, validation)
		},
	}

	cmd.Flags().StringVarP(&scenarioID, "scenario", "s", "", "Run only a specific scenario by ID")
	addJSONFlags(cmd, &jsonOut, &format)
	cmd.Flags().StringVar(&profile, "profile", config.TestProfileStandalone, "Execution profile for scenario requirements (standalone|local|ci|prod)")
	// @AX:NOTE [AUTO] @AX:REASON: magic constant — 30s default timeout mirrors NewRunner default; keep in sync with pkg/e2e/runner.go
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Per-scenario timeout")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show stdout/stderr for each scenario")
	cmd.Flags().StringVar(&projectDir, "project-dir", ".", "Project root directory")
	cmd.Flags().StringVar(&validation, "scenario-validation", "warn", "Scenario admission mode (warn|enforce)")

	return cmd
}
