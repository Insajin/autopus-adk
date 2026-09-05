package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func executeLeadTime(t *testing.T, dir string, p leadTimeParams, jsonMode bool) (map[string]any, string, error) {
	t.Helper()
	cmd := newTelemetryLeadTimeCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := runTelemetryLeadTime(cmd, dir, p, jsonMode)
	if !jsonMode {
		return nil, out.String(), err
	}
	return decodeJSONMap(t, out.Bytes()), out.String(), err
}

func leadTimeData(t *testing.T, payload map[string]any) (map[string]any, map[string]any) {
	t.Helper()
	data, ok := payload["data"].(map[string]any)
	require.True(t, ok, "data object")
	report, ok := data["leadtime"].(map[string]any)
	require.True(t, ok, "leadtime object")
	return data, report
}

func TestTelemetryLeadTime_JSONReportsRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedLeadTimeRun(t, dir, "SPEC-LT-JSON", false, false)

	payload, raw, err := executeLeadTime(t, dir, leadTimeParams{}, true)
	require.NoError(t, err)
	assert.Equal(t, "ok", payload["status"])
	data, report := leadTimeData(t, payload)

	assert.Equal(t, "SPEC-LT-JSON", data["spec_id"])
	assert.NotContains(t, data, "baseline")
	assert.NotNil(t, report["time_to_first_slice_ns"])
	assert.Equal(t, []any{"plan", "build_a", "integration"}, report["critical_path"])
	assert.Equal(t, map[string]any{"context lost": float64(1)}, report["rereads_by_reason"])
	assert.Equal(t, map[string]any{"integration": map[string]any{"count": float64(1), "files_touched": float64(2)}}, report["defects_by_discovery_phase"])
	assert.Equal(t, []any{"data_loss"}, report["unresolved_safety_gates"])
	assert.Equal(t, float64(0), report["escaped_defects"])
	estimate, ok := report["estimate_vs_actual"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, estimate, "within_range")
	assert.NotContains(t, raw, "pkg/a.go", "targets are never emitted")
}

func TestTelemetryLeadTime_TextReportsRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedLeadTimeRun(t, dir, "SPEC-LT-TEXT", true, false)

	_, out, err := executeLeadTime(t, dir, leadTimeParams{runID: "SPEC-LT-TEXT"}, false)
	require.NoError(t, err)
	assert.Contains(t, out, "## Lead Time")
	assert.Contains(t, out, "Critical path: plan -> build_a -> integration")
	assert.Contains(t, out, "Unresolved safety gates: none")
	assert.NotContains(t, out, "Baseline")
}

func TestTelemetryLeadTime_BaselineNoRegression(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedLeadTimeRun(t, dir, "SPEC-LT-BASE", false, false)
	seedLeadTimeRun(t, dir, "SPEC-LT-BASE", true, false)

	payload, _, err := executeLeadTime(t, dir, leadTimeParams{runID: "SPEC-LT-BASE", baseline: "SPEC-LT-BASE"}, true)
	require.NoError(t, err)
	assert.Equal(t, "ok", payload["status"])
	data, report := leadTimeData(t, payload)
	baseline, ok := data["baseline"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, baseline["regression"])
	assert.Equal(t, float64(-1), baseline["unresolved_safety_gates_delta"])
	assert.NotNil(t, baseline["delta_first_slice_ns"])
	assert.Contains(t, baseline, "delta_completion_ns")
	assert.Equal(t, []any{}, report["unresolved_safety_gates"])
}

func TestTelemetryLeadTime_BaselineRegressionExitsNonZero(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedLeadTimeRun(t, dir, "SPEC-LT-REG", true, false)
	seedLeadTimeRun(t, dir, "SPEC-LT-REG", false, true)

	payload, _, err := executeLeadTime(t, dir, leadTimeParams{baseline: "SPEC-LT-REG"}, true)
	require.Error(t, err)
	assert.True(t, isJSONFatalError(err), "regression must surface as a non-zero exit")
	assert.Equal(t, "error", payload["status"])
	errorPayload, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "leadtime_regression", errorPayload["code"])
	data, _ := leadTimeData(t, payload)
	baseline := data["baseline"].(map[string]any)
	assert.Equal(t, true, baseline["regression"])
	assert.Equal(t, []any{"escaped_defects increased 0 -> 1", "unresolved_safety_gates increased 0 -> 1"}, baseline["regression_reasons"])

	_, out, err := executeLeadTime(t, dir, leadTimeParams{baseline: "SPEC-LT-REG"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regression versus baseline SPEC-LT-REG")
	assert.Contains(t, out, "Regression: true")
}

func TestTelemetryLeadTime_BaselineDirectoryAndOtherSpec(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedLeadTimeRun(t, dir, "SPEC-LT-CUR", true, false)
	other := t.TempDir()
	seedLeadTimeRun(t, other, "SPEC-LT-OLD", false, false)

	payload, _, err := executeLeadTime(t, dir, leadTimeParams{baseline: other}, true)
	require.NoError(t, err)
	data, _ := leadTimeData(t, payload)
	assert.Equal(t, "SPEC-LT-OLD", data["baseline"].(map[string]any)["baseline_spec_id"])

	seedLeadTimeRun(t, dir, "SPEC-LT-PEER", false, false)
	payload, _, err = executeLeadTime(t, dir, leadTimeParams{runID: "SPEC-LT-CUR", baseline: "SPEC-LT-PEER"}, true)
	require.NoError(t, err)
	data, _ = leadTimeData(t, payload)
	assert.Equal(t, "SPEC-LT-PEER", data["baseline"].(map[string]any)["baseline_spec_id"])
}

func TestTelemetryLeadTime_UnavailableRunsAreJSONErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	payload, _, err := executeLeadTime(t, dir, leadTimeParams{}, true)
	require.Error(t, err)
	assert.True(t, isJSONFatalError(err))
	assert.Equal(t, "telemetry_leadtime_unavailable", payload["error"].(map[string]any)["code"])

	seedLeadTimeRun(t, dir, "SPEC-LT-ONLY", true, false)
	payload, _, err = executeLeadTime(t, dir, leadTimeParams{baseline: "SPEC-LT-ONLY"}, true)
	require.Error(t, err)
	assert.Equal(t, "telemetry_leadtime_baseline_unavailable", payload["error"].(map[string]any)["code"])

	_, _, err = executeLeadTime(t, dir, leadTimeParams{baseline: "SPEC-NOPE"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no baseline runs found")
}

func TestTelemetryLeadTimeCmd_RegisteredUnderTelemetry(t *testing.T) {
	dir := t.TempDir()
	seedLeadTimeRun(t, dir, "SPEC-LT-ROOT", true, false)

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	require.NoError(t, os.Chdir(dir))

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"telemetry", "leadtime", "--json"})
	require.NoError(t, root.Execute())

	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "auto telemetry leadtime")
	_, report := leadTimeData(t, payload)
	assert.Equal(t, []any{}, report["unresolved_safety_gates"])
}
