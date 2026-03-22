package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

// writePipelineRun seeds a JSONL file under baseDir with a complete pipeline_end event.
func writePipelineRun(t *testing.T, baseDir string, run telemetry.PipelineRun) {
	t.Helper()

	rec, err := telemetry.NewRecorder(baseDir, run.SpecID)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	rec.StartPipeline(run.SpecID, run.QualityMode)
	for _, ph := range run.Phases {
		rec.StartPhase(ph.Name)
		for _, ag := range ph.Agents {
			rec.RecordAgent(ag)
		}
		rec.EndPhase(ph.Status)
	}
	_ = rec.Finalize(run.FinalStatus)
}

func TestTelemetrySummaryCmd(t *testing.T) {
	dir := t.TempDir()
	writePipelineRun(t, dir, telemetry.PipelineRun{
		SpecID:        "SPEC-001",
		FinalStatus:   "PASS",
		QualityMode:   "balanced",
		TotalDuration: 5 * time.Second,
	})

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"telemetry", "summary"})

	// Override cwd by changing directory temporarily.
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "SPEC-001") {
		t.Errorf("summary output missing spec ID; got:\n%s", buf.String())
	}
}

func TestTelemetryCostCmd(t *testing.T) {
	dir := t.TempDir()
	writePipelineRun(t, dir, telemetry.PipelineRun{
		SpecID:      "SPEC-002",
		FinalStatus: "PASS",
		QualityMode: "balanced",
	})

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"telemetry", "cost"})

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "Cost Report") {
		t.Errorf("cost output missing header; got:\n%s", buf.String())
	}
}

func TestTelemetryCompareCmd(t *testing.T) {
	dir := t.TempDir()
	writePipelineRun(t, dir, telemetry.PipelineRun{
		SpecID:      "SPEC-003",
		FinalStatus: "PASS",
		QualityMode: "balanced",
	})
	writePipelineRun(t, dir, telemetry.PipelineRun{
		SpecID:      "SPEC-004",
		FinalStatus: "FAIL",
		QualityMode: "ultra",
	})

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"telemetry", "compare"})

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "Comparison") {
		t.Errorf("compare output missing header; got:\n%s", buf.String())
	}
}

func TestTelemetryRecordStart(t *testing.T) {
	dir := t.TempDir()
	err := runTelemetryRecord(dir, recordParams{
		specID:      "SPEC-REC",
		action:      "start",
		qualityMode: "balanced",
		phase:       "plan",
	})
	if err != nil {
		t.Fatalf("record start: %v", err)
	}
	// Verify a JSONL file was created.
	matches, _ := filepath.Glob(filepath.Join(dir, ".autopus", "telemetry", "*.jsonl"))
	if len(matches) == 0 {
		t.Error("expected JSONL file, none found")
	}
}

func TestTelemetryRecordAgent(t *testing.T) {
	dir := t.TempDir()
	err := runTelemetryRecord(dir, recordParams{
		specID:      "SPEC-REC",
		action:      "agent",
		agent:       "executor",
		phase:       "execute",
		status:      "PASS",
		files:       3,
		tokens:      1200,
		qualityMode: "balanced",
	})
	if err != nil {
		t.Fatalf("record agent: %v", err)
	}
}

func TestTelemetryRecordEnd(t *testing.T) {
	dir := t.TempDir()
	err := runTelemetryRecord(dir, recordParams{
		specID: "SPEC-REC",
		action: "end",
		status: "PASS",
	})
	if err != nil {
		t.Fatalf("record end: %v", err)
	}
}

func TestTelemetryRecordUnknownAction(t *testing.T) {
	dir := t.TempDir()
	err := runTelemetryRecord(dir, recordParams{
		specID: "SPEC-REC",
		action: "bogus",
	})
	if err == nil {
		t.Fatal("expected error for unknown action, got nil")
	}
}

func TestTelemetryRecordMissingSpecID(t *testing.T) {
	dir := t.TempDir()
	for _, action := range []string{"start", "agent", "end"} {
		err := runTelemetryRecord(dir, recordParams{action: action, agent: "x"})
		if err == nil {
			t.Errorf("action %q: expected error for missing spec-id", action)
		}
	}
}

func TestResolveSingleRunNoRuns(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveSingleRun(dir, "")
	if err == nil {
		t.Fatal("expected error when no runs exist")
	}
}

func TestResolveSingleRunBySpecID(t *testing.T) {
	dir := t.TempDir()
	writePipelineRun(t, dir, telemetry.PipelineRun{
		SpecID:      "SPEC-FILTER",
		FinalStatus: "PASS",
		QualityMode: "balanced",
	})

	run, err := resolveSingleRun(dir, "SPEC-FILTER")
	if err != nil {
		t.Fatalf("resolveSingleRun: %v", err)
	}
	if run.SpecID != "SPEC-FILTER" {
		t.Errorf("want SPEC-FILTER, got %s", run.SpecID)
	}
}

func TestResolveSingleRunBySpecIDNotFound(t *testing.T) {
	dir := t.TempDir()
	writePipelineRun(t, dir, telemetry.PipelineRun{
		SpecID:      "SPEC-X",
		FinalStatus: "PASS",
		QualityMode: "balanced",
	})

	_, err := resolveSingleRun(dir, "SPEC-MISSING")
	if err == nil {
		t.Fatal("expected error for missing spec-id")
	}
}

func TestResolveTwoRunsInsufficient(t *testing.T) {
	dir := t.TempDir()
	writePipelineRun(t, dir, telemetry.PipelineRun{
		SpecID:      "SPEC-ONE",
		FinalStatus: "PASS",
		QualityMode: "balanced",
	})
	_, err := resolveTwoRuns(dir, "")
	if err == nil {
		t.Fatal("expected error when fewer than 2 runs exist")
	}
}

func TestResolveTwoRunsBySpecID(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		writePipelineRun(t, dir, telemetry.PipelineRun{
			SpecID:      "SPEC-PAIR",
			FinalStatus: "PASS",
			QualityMode: "balanced",
		})
	}

	runs, err := resolveTwoRuns(dir, "SPEC-PAIR")
	if err != nil {
		t.Fatalf("resolveTwoRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("want 2 runs, got %d", len(runs))
	}
}
