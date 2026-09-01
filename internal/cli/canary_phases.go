package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// canaryNotReached is the reason recorded for checks that a bail-out skipped.
// Canary used to seed build/e2e/doctor with "PASS" and return on the first
// failure, so a run that stopped at the first build target still reported
// "e2e: PASS" and "doctor: PASS" for commands that never executed.
func canaryNotReached(id string) string {
	return "not reached: " + id + " failed"
}

func canaryMarkNotReached(result *canaryResult, id string, areas ...string) {
	for _, area := range areas {
		result.Skipped = append(result.Skipped, canarySkippedCheck{area, canaryNotReached(id)})
	}
}

// runCanaryBuildPhase builds every target detection resolved. No detected
// target is a stated skip, not a failure: `auto qa init` wires `auto canary`
// into the canary-explicit lane for every project, so a project with no build
// step must still be able to satisfy that lane.
func runCanaryBuildPhase(ctx context.Context, projectDir string, targets []canaryCommandTarget, result *canaryResult) error {
	if len(targets) == 0 {
		result.Skipped = append(result.Skipped, canarySkippedCheck{"build", "no buildable stack detected in " + projectDir})
		return nil
	}
	for _, target := range targets {
		run := runCanaryExternal(ctx, target.ID, target.Command, target.Dir, target.Args...)
		result.Targets = append(result.Targets, run)
		if run.Status != "PASS" {
			result.Build = "FAIL"
			result.Verdict = "FAIL"
			canaryMarkNotReached(result, run.ID, "e2e", "doctor", "endpoint", "browser")
			return fmt.Errorf("%s failed: %s", run.ID, firstCanaryLine(run.Detail))
		}
	}
	result.Build = "PASS"
	return nil
}

// runCanaryHarnessPhase runs the ADK-side checks: the version smoke scenario and
// `auto doctor`.
func runCanaryHarnessPhase(ctx context.Context, projectDir string, result *canaryResult) error {
	exe, _ := os.Executable()
	if exe == "" {
		exe = "auto"
	}
	e2eArgs := []string{"test", "run", "--project-dir", projectDir, "--scenario", "version", "--format", "json", "--timeout", "60s"}
	run := runCanaryExternal(ctx, "e2e-version", canaryDisplay(exe, e2eArgs), projectDir, append([]string{exe}, e2eArgs...)...)
	result.Targets = append(result.Targets, run)
	if run.Status != "PASS" {
		result.E2E = "FAIL"
		result.Verdict = "FAIL"
		canaryMarkNotReached(result, run.ID, "doctor", "endpoint", "browser")
		return fmt.Errorf("%s failed: %s", run.ID, firstCanaryLine(run.Detail))
	}
	result.E2E = "PASS"

	if reason := canaryDoctorSkip(projectDir); reason != "" {
		result.Skipped = append(result.Skipped, canarySkippedCheck{"doctor", reason})
		return nil
	}
	run = runCanaryExternal(ctx, "doctor", canaryDisplay(exe, []string{"doctor"}), projectDir, exe, "doctor")
	result.Targets = append(result.Targets, run)
	if run.Status != "PASS" {
		result.Doctor = "FAIL"
		result.Verdict = "FAIL"
		canaryMarkNotReached(result, run.ID, "endpoint", "browser")
		return fmt.Errorf("%s failed: %s", run.ID, firstCanaryLine(run.Detail))
	}
	result.Doctor = "PASS"
	return nil
}

func canaryDisplay(exe string, args []string) string {
	return exe + " " + strings.Join(args, " ")
}

// canaryDoctorSkip states why `auto doctor` cannot run here, or "" when it can.
// `auto doctor` audits the harness wiring of an Autopus-managed project and
// refuses a directory without autopus.yaml. Reporting that refusal as a canary
// failure made the canary-explicit lane unsatisfiable for every project that
// has not adopted the full harness - which is every third-party project that
// only ran `auto qa init`.
func canaryDoctorSkip(projectDir string) string {
	if canaryFileExists(projectDir, "autopus.yaml") {
		return ""
	}
	return "not an Autopus-managed project (no autopus.yaml)"
}
