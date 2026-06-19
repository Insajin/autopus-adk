package workflow

import (
	"context"
	"encoding/json"
	"testing"
)

// fakeRunner returns a fixed exit code per command, keyed by whether the
// command line contains "build" or "test".
type fakeRunner struct {
	buildExit int
	testExit  int
}

func (f fakeRunner) Run(_ context.Context, name string, args ...string) (int, error) {
	all := append([]string{name}, args...)
	for _, a := range all {
		if a == "build" {
			return f.buildExit, nil
		}
		if a == "test" {
			return f.testExit, nil
		}
	}
	return 0, nil
}

// S8: a fake CommandRunner returning build exit=1 yields verdict "fail" derived
// from the exit code, with verdict_source "exit_code".
func TestEvaluate_BuildExitOneFailsFromExitCode(t *testing.T) {
	got := EvaluateGate(context.Background(), fakeRunner{buildExit: 1, testExit: 0},
		[]string{"go", "build", "./..."}, []string{"go", "test", "./..."})

	if got.Verdict != "fail" {
		t.Fatalf("verdict = %q, want fail", got.Verdict)
	}
	if got.VerdictSource != VerdictSourceExitCode {
		t.Fatalf("verdict_source = %q, want %q", got.VerdictSource, VerdictSourceExitCode)
	}
	if got.BuildExit != 1 || got.TestExit != 0 {
		t.Fatalf("build_exit=%d test_exit=%d, want 1/0", got.BuildExit, got.TestExit)
	}
}

// S8 (pass branch): build=0 and test=0 yields verdict "pass".
func TestEvaluate_AllZeroPasses(t *testing.T) {
	got := EvaluateGate(context.Background(), fakeRunner{buildExit: 0, testExit: 0},
		[]string{"go", "build", "./..."}, []string{"go", "test", "./..."})

	if got.Verdict != "pass" {
		t.Fatalf("verdict = %q, want pass", got.Verdict)
	}
	if got.VerdictSource != VerdictSourceExitCode {
		t.Fatalf("verdict_source = %q, want %q", got.VerdictSource, VerdictSourceExitCode)
	}
}

func TestGateResult_EncodeJSON(t *testing.T) {
	r := GateResult{Verdict: "fail", VerdictSource: VerdictSourceExitCode, BuildExit: 1, TestExit: 0}
	data, err := r.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["verdict"] != "fail" || decoded["verdict_source"] != "exit_code" {
		t.Fatalf("unexpected json: %s", data)
	}
}
