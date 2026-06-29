package workflow

import (
	"context"
	"regexp"
	"strconv"
)

// CoverageRunner is distinct from the exit-code-only CommandRunner.
// It returns stdout in addition to the exit code and error.
type CoverageRunner interface {
	RunOutput(ctx context.Context, name string, args ...string) (stdout string, exitCode int, err error)
}

var (
	totalRegex    = regexp.MustCompile(`total:\s*\(statements\)\s*([0-9.]+)%`)
	coverageRegex = regexp.MustCompile(`coverage:\s*([0-9.]+)%\s*of\s*statements`)
)

// ParseCoverage extracts the coverage percentage from stdout.
func ParseCoverage(stdout string) (float64, bool) {
	if matches := totalRegex.FindStringSubmatch(stdout); len(matches) > 1 {
		val, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			return val, true
		}
	}
	if matches := coverageRegex.FindStringSubmatch(stdout); len(matches) > 1 {
		val, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			return val, true
		}
	}
	return 0.0, false
}

// EvaluateCoverageGate parses the measured coverage percentage from stdout,
// compares it to the threshold, and returns a GateResult.
func EvaluateCoverageGate(ctx context.Context, runner CoverageRunner, coverageCmd []string, threshold int) GateResult {
	if len(coverageCmd) == 0 {
		return GateResult{
			Verdict:       "fail",
			VerdictSource: VerdictSourceExitCode,
			BuildExit:     0,
			TestExit:      1,
		}
	}

	stdout, exitCode, err := runner.RunOutput(ctx, coverageCmd[0], coverageCmd[1:]...)
	if err != nil {
		if exitCode == 0 {
			exitCode = 1
		}
		return GateResult{
			Verdict:       "fail",
			VerdictSource: VerdictSourceExitCode,
			BuildExit:     0,
			TestExit:      exitCode,
		}
	}

	cov, ok := ParseCoverage(stdout)
	if !ok {
		return GateResult{
			Verdict:       "fail",
			VerdictSource: VerdictSourceExitCode,
			BuildExit:     0,
			TestExit:      1,
		}
	}

	verdict := "fail"
	testExit := 1
	if cov >= float64(threshold) {
		verdict = "pass"
		testExit = 0
	}

	return GateResult{
		Verdict:       verdict,
		VerdictSource: VerdictSourceExitCode,
		BuildExit:     0,
		TestExit:      testExit,
	}
}
