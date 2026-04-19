package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/e2e"
)

// newAutoTestCmd creates the `auto test` parent command with the `run` subcommand.
func newAutoTestCmd() *cobra.Command {
	parent := &cobra.Command{
		Use:   "test",
		Short: "Run E2E scenarios against the project",
	}

	parent.AddCommand(newAutoTestRunCmd())
	return parent
}

// newAutoTestRunCmd creates the `auto test run` subcommand.
func newAutoTestRunCmd() *cobra.Command {
	var (
		scenarioID string
		jsonOut    bool
		profile    string
		timeout    time.Duration
		verbose    bool
		projectDir string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute E2E scenarios and report PASS/FAIL per scenario",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAutoTest(cmd, scenarioID, jsonOut, profile, timeout, verbose, projectDir)
		},
	}

	cmd.Flags().StringVarP(&scenarioID, "scenario", "s", "", "Run only a specific scenario by ID")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output results as JSON")
	cmd.Flags().StringVar(&profile, "profile", config.TestProfileStandalone, "Execution profile for scenario requirements (standalone|local|ci|prod)")
	// @AX:NOTE [AUTO] @AX:REASON: magic constant — 30s default timeout mirrors NewRunner default; keep in sync with pkg/e2e/runner.go
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Per-scenario timeout")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show stdout/stderr for each scenario")
	cmd.Flags().StringVar(&projectDir, "project-dir", ".", "Project root directory")

	return cmd
}

// scenarioJSONResult is the JSON-serializable result for one scenario.
type scenarioJSONResult struct {
	ID      string  `json:"id"`
	Status  string  `json:"status"` // PASS | FAIL | SKIP
	Elapsed float64 `json:"elapsed_seconds"`
	Reason  string  `json:"reason,omitempty"`
}

// @AX:NOTE [AUTO] @AX:REASON: design choice — command strips markdown backticks from Command field at runtime (line 124); scenarios.md stores commands as inline code e.g. "`auto init`"
// runAutoTest executes the test run logic.
func runAutoTest(cmd *cobra.Command, scenarioID string, jsonOut bool, profile string, timeout time.Duration, verbose bool, projectDir string) error {
	out := cmd.OutOrStdout()
	profile = strings.ToLower(strings.TrimSpace(profile))
	if !config.IsValidTestProfile(profile) {
		return fmt.Errorf("invalid profile %q: must be standalone, local, ci, or prod", profile)
	}

	cfg, err := config.Load(projectDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	availableCapabilities := cfg.AvailableTestCapabilities(profile)

	// Read scenarios.md from .autopus/project/scenarios.md.
	scenariosPath := filepath.Join(projectDir, ".autopus", "project", "scenarios.md")
	data, err := os.ReadFile(scenariosPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(out, "no scenarios found (missing scenarios.md)")
			if jsonOut {
				enc := json.NewEncoder(out)
				return enc.Encode(map[string]interface{}{"results": []scenarioJSONResult{}})
			}
			return nil
		}
		return fmt.Errorf("read scenarios.md: %w", err)
	}

	set, err := e2e.ParseScenarios(data)
	if err != nil {
		return fmt.Errorf("parse scenarios: %w", err)
	}

	if len(set.Scenarios) == 0 {
		fmt.Fprintln(out, "no scenarios found")
		if jsonOut {
			enc := json.NewEncoder(out)
			return enc.Encode(map[string]interface{}{"results": []scenarioJSONResult{}})
		}
		return nil
	}

	// Resolve build configuration from scenario set.
	// Multi-build (Builds) takes precedence; legacy single BuildCommand as fallback.
	buildCmd := set.Build
	autoBuild := len(set.Builds) > 0 || buildCmd != ""

	runnerOpts := e2e.RunnerOptions{
		ProjectDir:   projectDir,
		AutoBuild:    autoBuild,
		BuildCommand: buildCmd,
		Builds:       set.Builds,
		Timeout:      timeout,
	}
	runner := e2e.NewRunner(runnerOpts)

	var (
		results              []scenarioJSONResult
		passed, run, skipped int
	)

	for _, s := range set.Scenarios {
		// Skip non-active or filtered scenarios.
		if s.Status == "deprecated" || s.Status == "skip" {
			continue
		}
		if scenarioID != "" && s.ID != scenarioID {
			continue
		}

		run++
		missingRequirements := e2e.MissingScenarioRequirements(s, availableCapabilities)
		if len(missingRequirements) > 0 {
			skipped++
			jr := scenarioJSONResult{
				ID:     fmt.Sprintf("S%d", s.Number),
				Status: "SKIP",
				Reason: fmt.Sprintf("requires %s (profile=%s)", strings.Join(missingRequirements, ", "), profile),
			}
			results = append(results, jr)
			if !jsonOut {
				fmt.Fprintf(out, "%-24s SKIP  %s\n", fmt.Sprintf("S%d: %s", s.Number, s.ID), jr.Reason)
			}
			continue
		}

		// Strip surrounding backticks from command field (markdown inline code format).
		s.Command = strings.Trim(s.Command, "`")
		start := time.Now()
		res, err := runner.Run(s)
		elapsed := time.Since(start).Seconds()

		jr := scenarioJSONResult{
			ID:      fmt.Sprintf("S%d", s.Number),
			Elapsed: elapsed,
		}

		if err != nil {
			jr.Status = "FAIL"
			jr.Reason = err.Error()
		} else if res.Pass {
			jr.Status = "PASS"
			passed++
		} else {
			jr.Status = "FAIL"
			jr.Reason = res.FailureDetails
		}

		results = append(results, jr)

		if !jsonOut {
			label := fmt.Sprintf("S%d: %s", s.Number, s.ID)
			if jr.Status == "PASS" {
				fmt.Fprintf(out, "%-24s PASS  (%.2fs)\n", label, elapsed)
			} else {
				fmt.Fprintf(out, "%-24s FAIL  %s\n", label, jr.Reason)
			}

			if verbose && res != nil {
				if res.Stdout != "" {
					fmt.Fprintf(out, "  stdout: %s\n", res.Stdout)
				}
				if res.Stderr != "" {
					fmt.Fprintf(out, "  stderr: %s\n", res.Stderr)
				}
			}
		}
	}

	if jsonOut {
		enc := json.NewEncoder(out)
		return enc.Encode(map[string]interface{}{"results": results})
	}

	failed := run - passed - skipped
	if skipped == 0 {
		fmt.Fprintf(out, "\nResults: %d/%d passed\n", passed, run)
	} else {
		fmt.Fprintf(out, "\nResults: %d passed, %d skipped, %d failed\n", passed, skipped, failed)
	}

	if failed > 0 {
		return fmt.Errorf("%d scenario(s) failed", failed)
	}
	return nil
}
