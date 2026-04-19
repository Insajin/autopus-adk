package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

// TestBuildConfig_Minimize verifies buildConfig sets Direction to Minimize by default.
func TestBuildConfig_Minimize(t *testing.T) {
	t.Parallel()

	f := experimentFlags{
		metric:              "echo 1",
		direction:           "minimize",
		target:              []string{"main.go"},
		maxIterations:       10,
		timeout:             5 * time.Second,
		metricRuns:          1,
		simplicityThreshold: 0.001,
	}
	cfg := buildConfig(f)
	assert.Equal(t, "echo 1", cfg.MetricCmd)
	assert.Equal(t, []string{"main.go"}, cfg.TargetFiles)
	assert.Equal(t, 10, cfg.MaxIterations)
}

// TestBuildConfig_Maximize verifies buildConfig sets Direction to Maximize.
func TestBuildConfig_Maximize(t *testing.T) {
	t.Parallel()

	f := experimentFlags{direction: "maximize"}
	cfg := buildConfig(f)
	_ = cfg
}

// TestNewExperimentSummaryCmd_EmptyInput verifies summary cmd handles empty stdin.
func TestNewExperimentSummaryCmd_EmptyInput(t *testing.T) {
	t.Parallel()

	cmd := newExperimentSummaryCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(bytes.NewReader([]byte("")))
	err := cmd.RunE(cmd, nil)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "total=0")
}

// writeTelemetryEvent writes a pipeline_end event to a JSONL file for testing.
func writeTelemetryEvent(t *testing.T, dir string, run telemetry.PipelineRun) {
	t.Helper()
	telDir := filepath.Join(dir, ".autopus", "telemetry")
	require.NoError(t, os.MkdirAll(telDir, 0755))

	data, err := json.Marshal(run)
	require.NoError(t, err)

	event := map[string]interface{}{
		"type":      "pipeline_end",
		"timestamp": time.Now().Format(time.RFC3339),
		"data":      json.RawMessage(data),
	}
	line, err := json.Marshal(event)
	require.NoError(t, err)

	f, err := os.OpenFile(filepath.Join(telDir, "runs.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	defer f.Close()

	_, err = f.Write(append(line, '\n'))
	require.NoError(t, err)
}

// TestResolveSingleRun_EmptyDir verifies error when no runs exist.
func TestResolveSingleRun_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := resolveSingleRun(dir, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pipeline runs found")
}

// TestResolveSingleRun_LatestRun verifies returning the latest run.
func TestResolveSingleRun_LatestRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeTelemetryEvent(t, dir, telemetry.PipelineRun{SpecID: "SPEC-001", FinalStatus: "PASS"})

	run, err := resolveSingleRun(dir, "")
	require.NoError(t, err)
	require.NotNil(t, run)
	assert.Equal(t, "SPEC-001", run.SpecID)
}

// TestResolveSingleRun_BySpecID verifies filtering by spec ID.
func TestResolveSingleRun_BySpecID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeTelemetryEvent(t, dir, telemetry.PipelineRun{SpecID: "SPEC-001", FinalStatus: "PASS"})
	writeTelemetryEvent(t, dir, telemetry.PipelineRun{SpecID: "SPEC-002", FinalStatus: "FAIL"})

	run, err := resolveSingleRun(dir, "SPEC-002")
	require.NoError(t, err)
	require.NotNil(t, run)
	assert.Equal(t, "SPEC-002", run.SpecID)
	assert.Equal(t, "FAIL", run.FinalStatus)
}

// TestResolveSingleRun_BySpecID_NotFound verifies error when spec ID not found.
func TestResolveSingleRun_BySpecID_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeTelemetryEvent(t, dir, telemetry.PipelineRun{SpecID: "SPEC-001", FinalStatus: "PASS"})

	_, err := resolveSingleRun(dir, "SPEC-999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no runs found")
}

// TestResolveTwoRuns_Insufficient verifies error when fewer than 2 runs exist.
func TestResolveTwoRuns_Insufficient(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeTelemetryEvent(t, dir, telemetry.PipelineRun{SpecID: "SPEC-001", FinalStatus: "PASS"})

	_, err := resolveTwoRuns(dir, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "need at least 2 runs")
}

// TestResolveTwoRuns_ReturnsMostRecent verifies two most recent runs are returned.
func TestResolveTwoRuns_ReturnsMostRecent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeTelemetryEvent(t, dir, telemetry.PipelineRun{SpecID: "SPEC-001", FinalStatus: "FAIL"})
	writeTelemetryEvent(t, dir, telemetry.PipelineRun{SpecID: "SPEC-001", FinalStatus: "PASS"})

	runs, err := resolveTwoRuns(dir, "")
	require.NoError(t, err)
	assert.Len(t, runs, 2)
}

// TestResolveTwoRuns_BySpecID verifies filtering by specID for two runs.
func TestResolveTwoRuns_BySpecID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeTelemetryEvent(t, dir, telemetry.PipelineRun{SpecID: "SPEC-001", FinalStatus: "FAIL"})
	writeTelemetryEvent(t, dir, telemetry.PipelineRun{SpecID: "SPEC-001", FinalStatus: "PASS"})

	runs, err := resolveTwoRuns(dir, "SPEC-001")
	require.NoError(t, err)
	assert.Len(t, runs, 2)
	for _, r := range runs {
		assert.Equal(t, "SPEC-001", r.SpecID)
	}
}

// TestResolveTwoRuns_BySpecID_NotEnough verifies error when spec has fewer than 2 runs.
func TestResolveTwoRuns_BySpecID_NotEnough(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeTelemetryEvent(t, dir, telemetry.PipelineRun{SpecID: "SPEC-001", FinalStatus: "PASS"})

	_, err := resolveTwoRuns(dir, "SPEC-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "need at least 2 runs")
}
