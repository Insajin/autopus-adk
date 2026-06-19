package workflow

import (
	"context"
	"encoding/json"
)

// VerdictSourceExitCode is the canonical verdict source for the deterministic
// gate: the verdict derives from build/test command exit codes, never from an
// LLM verdict.
const VerdictSourceExitCode = "exit_code"

// CommandRunner is the injectable seam the deterministic gate uses to run the
// build and test commands. The production implementation wraps exec; tests
// inject a fake returning fixed exit codes. This keeps the gate independent of
// pkg/pipeline.PhaseBackend for exit-code data (REQ-011).
type CommandRunner interface {
	// Run executes name with args and returns the process exit code. err is
	// non-nil when the command could not be started or exited non-zero; the
	// exit code remains authoritative for the verdict.
	Run(ctx context.Context, name string, args ...string) (exitCode int, err error)
}

// GateResult is the structured verdict the `auto workflow gate` CLI emits for
// the workflow JS to read and branch on.
type GateResult struct {
	Verdict       string `json:"verdict"`
	VerdictSource string `json:"verdict_source"`
	BuildExit     int    `json:"build_exit"`
	TestExit      int    `json:"test_exit"`
}

// EvaluateGate runs the build and test commands through the runner seam and
// derives a deterministic verdict from their exit codes. VerdictSource is always
// "exit_code"; Verdict is "pass" iff both exit codes are 0, else "fail".
//
// Named EvaluateGate (not Evaluate) because the doctor capability evaluator
// EvaluateCapabilities shares this package; Go forbids two same-named funcs.
func EvaluateGate(ctx context.Context, runner CommandRunner, buildCmd, testCmd []string) GateResult {
	buildExit := runOne(ctx, runner, buildCmd)
	testExit := runOne(ctx, runner, testCmd)

	verdict := "fail"
	if buildExit == 0 && testExit == 0 {
		verdict = "pass"
	}
	return GateResult{
		Verdict:       verdict,
		VerdictSource: VerdictSourceExitCode,
		BuildExit:     buildExit,
		TestExit:      testExit,
	}
}

// runOne executes a single command and returns its exit code. An empty command
// is treated as success (exit 0). A start error with a zero exit code is
// normalized to 1 so a failed launch never masquerades as a passing gate.
func runOne(ctx context.Context, runner CommandRunner, cmd []string) int {
	if len(cmd) == 0 {
		return 0
	}
	exit, err := runner.Run(ctx, cmd[0], cmd[1:]...)
	if err != nil && exit == 0 {
		return 1
	}
	return exit
}

// EncodeJSON serializes the gate result for CLI stdout consumption.
func (r GateResult) EncodeJSON() ([]byte, error) {
	return json.Marshal(r)
}
