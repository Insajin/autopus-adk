package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/e2e"
)

// scenarioJSONResult is the JSON-serializable result for one scenario.
type scenarioJSONResult struct {
	ID      string  `json:"id"`
	Status  string  `json:"status"` // PASS | FAIL | SKIP
	Elapsed float64 `json:"elapsed_seconds"`
	Reason  string  `json:"reason,omitempty"`
}

// @AX:NOTE [AUTO]: Commands keep Markdown backticks in scenarios.md and strip them only immediately before runner dispatch.
// @AX:WARN [AUTO]: This command path has more than eight validation, quarantine, capability, execution, and output branches.
// @AX:REASON: Scenario admission must complete before runner construction so invalid input cannot trigger build or shell execution.
// runAutoTest executes the test run logic.
func runAutoTest(cmd *cobra.Command, scenarioID string, jsonOut bool, format string, profile string, timeout time.Duration, verbose bool, projectDir string, validation string) error {
	out := cmd.OutOrStdout()
	jsonMode, err := resolveJSONMode(jsonOut, format)
	if err != nil {
		return err
	}
	validation = strings.ToLower(strings.TrimSpace(validation))
	if validation != "warn" && validation != "enforce" {
		return fmt.Errorf("invalid scenario validation mode %q: must be warn or enforce", validation)
	}
	profile = strings.ToLower(strings.TrimSpace(profile))
	if !config.IsValidTestProfile(profile) {
		return fmt.Errorf("invalid profile %q: must be standalone, local, ci, or prod", profile)
	}

	cfg, err := config.Load(projectDir)
	if err != nil {
		if jsonMode {
			return writeJSONResultAndExit(
				cmd,
				jsonStatusError,
				fmt.Errorf("load config: %w", err),
				"test_config_load_failed",
				map[string]any{"project_dir": projectDir},
				nil,
				nil,
			)
		}
		return fmt.Errorf("load config: %w", err)
	}
	availableCapabilities := cfg.AvailableTestCapabilities(profile)

	// Read scenarios.md from .autopus/project/scenarios.md.
	scenariosPath := filepath.Join(projectDir, ".autopus", "project", "scenarios.md")
	data, err := os.ReadFile(scenariosPath)
	if err != nil {
		if os.IsNotExist(err) {
			if jsonMode {
				return writeAutoTestJSON(cmd, nil, 0, 0, 0, nil, []jsonMessage{{
					Code:    "scenarios_missing",
					Message: "No scenarios found because .autopus/project/scenarios.md is missing.",
				}})
			}
			fmt.Fprintln(out, "no scenarios found (missing scenarios.md)")
			return nil
		}
		if jsonMode {
			return writeJSONResultAndExit(
				cmd,
				jsonStatusError,
				fmt.Errorf("read scenarios.md: %w", err),
				"test_scenarios_read_failed",
				map[string]any{"project_dir": projectDir},
				nil,
				nil,
			)
		}
		return fmt.Errorf("read scenarios.md: %w", err)
	}

	report, err := e2e.ValidateScenarios(data)
	if err != nil {
		if jsonMode {
			return writeJSONResultAndExit(
				cmd,
				jsonStatusError,
				fmt.Errorf("parse scenarios: %w", err),
				"test_scenarios_parse_failed",
				map[string]any{"project_dir": projectDir},
				nil,
				nil,
			)
		}
		return fmt.Errorf("parse scenarios: %w", err)
	}
	set := report.ScenarioSet
	invalid := report.InvalidCount()
	if !jsonMode {
		for _, issue := range report.Issues {
			fmt.Fprintf(out, "%s %s %s\n", issue.Code, issue.ScenarioRef, issue.Field)
		}
	}
	if validation == "enforce" && len(report.Issues) > 0 {
		cause := fmt.Errorf("scenario validation failed: %d invalid scenario(s)", invalid)
		if jsonMode {
			return writeAutoTestValidationFailureJSON(cmd, invalid, report.Issues, cause)
		}
		return cause
	}

	if len(set.Scenarios) == 0 {
		if len(report.Issues) > 0 {
			if jsonMode {
				return writeAutoTestJSONWithDiagnostics(
					cmd, nil, 0, 0, 0, invalid, report.Issues, nil,
					[]jsonMessage{{Code: "scenarios_invalid", Message: "No valid runnable scenarios were found."}},
				)
			}
			fmt.Fprintf(out, "0 runnable scenarios (%d invalid)\n", invalid)
			return nil
		}
		if jsonMode {
			return writeAutoTestJSON(cmd, nil, 0, 0, 0, nil, []jsonMessage{{
				Code:    "scenarios_empty",
				Message: "No runnable scenarios were found in scenarios.md.",
			}})
		}
		fmt.Fprintln(out, "no scenarios found")
		return nil
	}

	runnable := make([]e2e.Scenario, 0, len(report.Runnable))
	for _, scenario := range report.Runnable {
		if scenarioID == "" || scenario.ID == scenarioID {
			runnable = append(runnable, scenario)
		}
	}
	if len(runnable) == 0 {
		warnings := []jsonMessage{{
			Code:    "scenarios_zero_runnable",
			Message: "No valid runnable scenarios were found.",
		}}
		if jsonMode {
			return writeAutoTestJSONWithDiagnostics(
				cmd, nil, 0, 0, 0, invalid, report.Issues, nil, warnings,
			)
		}
		fmt.Fprintf(out, "0 runnable scenarios (%d invalid)\n", invalid)
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

	for _, s := range runnable {
		run++
		missingRequirements := e2e.MissingScenarioRequirements(s, availableCapabilities)
		if len(missingRequirements) > 0 {
			skipped++
			jr := scenarioJSONResult{
				ID:     s.DisplayRef(),
				Status: "SKIP",
				Reason: fmt.Sprintf("requires %s (profile=%s)", strings.Join(missingRequirements, ", "), profile),
			}
			results = append(results, jr)
			if !jsonMode {
				fmt.Fprintf(out, "%-24s SKIP  %s\n", fmt.Sprintf("%s: %s", s.DisplayRef(), s.ID), jr.Reason)
			}
			continue
		}

		// Strip surrounding backticks from command field (markdown inline code format).
		s.Command = strings.Trim(s.Command, "`")
		start := time.Now()
		res, err := runner.Run(s)
		elapsed := time.Since(start).Seconds()

		jr := scenarioJSONResult{
			ID:      s.DisplayRef(),
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

		if !jsonMode {
			label := fmt.Sprintf("%s: %s", s.DisplayRef(), s.ID)
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

	failed := run - passed - skipped
	if jsonMode {
		warnings := make([]jsonMessage, 0)
		if invalid > 0 {
			warnings = append(warnings, jsonMessage{
				Code:    "scenarios_invalid",
				Message: fmt.Sprintf("%d invalid scenario(s) were quarantined.", invalid),
			})
		}
		if skipped > 0 {
			warnings = append(warnings, jsonMessage{
				Code:    "scenarios_skipped",
				Message: "One or more scenarios were skipped due to missing capabilities or filters.",
			})
		}
		if failed > 0 {
			warnings = append(warnings, jsonMessage{
				Code:    "scenarios_failed",
				Message: fmt.Sprintf("%d scenario(s) failed.", failed),
			})
			return writeAutoTestJSONWithDiagnostics(
				cmd, results, passed, failed, skipped, invalid, report.Issues,
				fmt.Errorf("%d scenario(s) failed", failed), warnings,
			)
		}
		return writeAutoTestJSONWithDiagnostics(
			cmd, results, passed, failed, skipped, invalid, report.Issues, nil, warnings,
		)
	}

	if invalid > 0 {
		fmt.Fprintf(out, "\nResults: %d passed, %d skipped, %d failed, %d invalid\n", passed, skipped, failed, invalid)
	} else if skipped == 0 {
		fmt.Fprintf(out, "\nResults: %d/%d passed\n", passed, run)
	} else {
		fmt.Fprintf(out, "\nResults: %d passed, %d skipped, %d failed\n", passed, skipped, failed)
	}

	if failed > 0 {
		return fmt.Errorf("%d scenario(s) failed", failed)
	}
	return nil
}
