package workflow

import (
	"context"
	"errors"
	"testing"
)

type mockCoverageRunner struct {
	stdout   string
	exitCode int
	err      error
}

func (m *mockCoverageRunner) RunOutput(ctx context.Context, name string, args ...string) (string, int, error) {
	return m.stdout, m.exitCode, m.err
}

func TestEvaluateCoverageGate_S3(t *testing.T) {
	ctx := context.Background()
	cmd := []string{"go", "test", "-cover"}

	// 84.0% coverage -> fail
	runner84 := &mockCoverageRunner{
		stdout: "total: (statements) 84.0%",
	}
	res84 := EvaluateCoverageGate(ctx, runner84, cmd, 85)
	if res84.Verdict != "fail" {
		t.Errorf("expected verdict fail for 84%%, got %s", res84.Verdict)
	}
	if res84.VerdictSource != VerdictSourceExitCode {
		t.Errorf("expected verdict source %s, got %s", VerdictSourceExitCode, res84.VerdictSource)
	}

	// 85.0% coverage -> pass
	runner85 := &mockCoverageRunner{
		stdout: "total: (statements) 85.0%",
	}
	res85 := EvaluateCoverageGate(ctx, runner85, cmd, 85)
	if res85.Verdict != "pass" {
		t.Errorf("expected verdict pass for 85%%, got %s", res85.Verdict)
	}

	// 85.0001% coverage -> pass
	runner85_0001 := &mockCoverageRunner{
		stdout: "total: (statements) 85.0001%",
	}
	res85_0001 := EvaluateCoverageGate(ctx, runner85_0001, cmd, 85)
	if res85_0001.Verdict != "pass" {
		t.Errorf("expected verdict pass for 85.0001%%, got %s", res85_0001.Verdict)
	}

	// per-package format coverage: 85.0% of statements -> pass
	runnerPkg85 := &mockCoverageRunner{
		stdout: "ok  github.com/foo/bar  0.010s  coverage: 85.0% of statements",
	}
	resPkg85 := EvaluateCoverageGate(ctx, runnerPkg85, cmd, 85)
	if resPkg85.Verdict != "pass" {
		t.Errorf("expected verdict pass for package 85%%, got %s", resPkg85.Verdict)
	}
}

func TestEvaluateCoverageGate_S4(t *testing.T) {
	ctx := context.Background()
	cmd := []string{"go", "test", "-cover"}

	// Empty stdout -> fail
	runnerEmpty := &mockCoverageRunner{
		stdout: "",
	}
	resEmpty := EvaluateCoverageGate(ctx, runnerEmpty, cmd, 85)
	if resEmpty.Verdict != "fail" {
		t.Errorf("expected verdict fail for empty stdout, got %s", resEmpty.Verdict)
	}

	// Non-numeric output -> fail
	runnerBad := &mockCoverageRunner{
		stdout: "total: (statements) NaN%",
	}
	resBad := EvaluateCoverageGate(ctx, runnerBad, cmd, 85)
	if resBad.Verdict != "fail" {
		t.Errorf("expected verdict fail for NaN coverage, got %s", resBad.Verdict)
	}

	// Command execution error -> fail
	runnerErr := &mockCoverageRunner{
		stdout:   "",
		exitCode: 2,
		err:      errors.New("exec error"),
	}
	resErr := EvaluateCoverageGate(ctx, runnerErr, cmd, 85)
	if resErr.Verdict != "fail" {
		t.Errorf("expected verdict fail on error, got %s", resErr.Verdict)
	}
	if resErr.TestExit != 2 {
		t.Errorf("expected TestExit 2, got %d", resErr.TestExit)
	}
}
