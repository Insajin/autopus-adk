package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/internal/cli"
	"github.com/insajin/autopus-adk/pkg/workflow"
)

// fakeProber injects capability probe results for the doctor subcommand.
type fakeProber struct {
	unavailable map[string]bool
	version     string
}

func (f fakeProber) Probe(primitive string) bool { return !f.unavailable[primitive] }
func (f fakeProber) Version() string             { return f.version }

// fakeRunner injects fixed exit codes for the gate subcommand, keyed by whether
// the command line contains "build" or "test".
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

// runWorkflow builds an isolated root with the workflow command wired to the
// given seams, runs it with args, and returns stdout and the Execute error.
func runWorkflow(prober workflow.Prober, runner workflow.CommandRunner, args ...string) (string, error) {
	root := &cobra.Command{Use: "auto", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(cli.NewWorkflowCmd(prober, runner))
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// S4: doctor with a required primitive (schema) unavailable exits non-zero and
// reports schema unavailable+gating, overall fail.
func TestWorkflowDoctor_RequiredUnavailableExitsNonZero(t *testing.T) {
	out, err := runWorkflow(
		fakeProber{unavailable: map[string]bool{"schema": true}, version: "2.1.154"},
		nil, "workflow", "doctor")
	if err == nil {
		t.Fatal("expected non-zero exit (Execute error) when required primitive unavailable")
	}

	report := decodeReport(t, out)
	if report.Overall != "fail" {
		t.Fatalf("overall = %q, want fail", report.Overall)
	}
	schema := findStatus(t, report, "schema")
	if schema.Status != "unavailable" || !schema.Gating {
		t.Fatalf("schema = %+v, want unavailable+gating", schema)
	}
}

// S12: doctor with a version below the pin exits non-zero with overall fail.
func TestWorkflowDoctor_BelowMinVersionExitsNonZero(t *testing.T) {
	out, err := runWorkflow(fakeProber{version: "2.1.140"}, nil, "workflow", "doctor")
	if err == nil {
		t.Fatal("expected non-zero exit when version below pin")
	}
	report := decodeReport(t, out)
	if report.VersionOK {
		t.Fatal("version_ok must be false for 2.1.140")
	}
	if report.Overall != "fail" {
		t.Fatalf("overall = %q, want fail", report.Overall)
	}
}

// S14: doctor with an advisory primitive (budget) unavailable but all
// required available and version ok exits 0 with budget advisory + overall
// pass.
func TestWorkflowDoctor_AdvisoryUnavailableExitsZero(t *testing.T) {
	out, err := runWorkflow(
		fakeProber{unavailable: map[string]bool{"budget": true}, version: "2.1.154"},
		nil, "workflow", "doctor")
	if err != nil {
		t.Fatalf("expected zero exit, got error: %v", err)
	}
	report := decodeReport(t, out)
	if report.Overall != "pass" {
		t.Fatalf("overall = %q, want pass", report.Overall)
	}
	b := findStatus(t, report, "budget")
	if b.Status != "unavailable" || b.Gating {
		t.Fatalf("budget = %+v, want unavailable+advisory(non-gating)", b)
	}
}

// FIDELITY-001 F1: doctor with the required parallel primitive unavailable exits
// non-zero (fail-fast) so /auto go --team falls back to the safe Route A path
// rather than crashing mid-launch at parallel(...).
func TestWorkflowDoctor_ParallelUnavailableFailsGate(t *testing.T) {
	out, err := runWorkflow(
		fakeProber{unavailable: map[string]bool{"parallel": true}, version: "2.1.154"},
		nil, "workflow", "doctor")
	if err == nil {
		t.Fatal("expected non-zero exit when required primitive parallel is unavailable")
	}
	report := decodeReport(t, out)
	if report.Overall != "fail" {
		t.Fatalf("overall = %q, want fail", report.Overall)
	}
	p := findStatus(t, report, "parallel")
	if p.Status != "unavailable" || !p.Gating {
		t.Fatalf("parallel = %+v, want unavailable+gating(required)", p)
	}
}

// S16: gate with fake CommandRunner (build exit=1, test exit=0) prints verdict
// JSON fail/exit_code with build_exit=1, test_exit=0 and exits 0.
func TestWorkflowGate_EmitsExitCodeVerdictJSON(t *testing.T) {
	out, err := runWorkflow(nil, fakeRunner{buildExit: 1, testExit: 0}, "workflow", "gate")
	if err != nil {
		t.Fatalf("gate must exit 0 (verdict lives in JSON), got: %v", err)
	}

	var result workflow.GateResult
	if err := json.Unmarshal([]byte(firstLine(out)), &result); err != nil {
		t.Fatalf("decode gate json %q: %v", out, err)
	}
	if result.Verdict != "fail" {
		t.Fatalf("verdict = %q, want fail", result.Verdict)
	}
	if result.VerdictSource != "exit_code" {
		t.Fatalf("verdict_source = %q, want exit_code", result.VerdictSource)
	}
	if result.BuildExit != 1 || result.TestExit != 0 {
		t.Fatalf("build_exit=%d test_exit=%d, want 1/0", result.BuildExit, result.TestExit)
	}
}

func decodeReport(t *testing.T, out string) workflow.CapabilityReport {
	t.Helper()
	var report workflow.CapabilityReport
	if err := json.Unmarshal([]byte(firstLine(out)), &report); err != nil {
		t.Fatalf("decode capability report %q: %v", out, err)
	}
	return report
}

func findStatus(t *testing.T, r workflow.CapabilityReport, name string) workflow.PrimitiveStatus {
	t.Helper()
	for _, p := range r.Primitives {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("primitive %q not in report", name)
	return workflow.PrimitiveStatus{}
}

// firstLine returns the first newline-delimited line (the JSON payload) so a
// trailing cobra error line does not break decoding.
func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
