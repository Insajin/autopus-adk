package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunTelemetryRecord_AgentUsageJSONRecordsExactlyOneValidatedReceipt(t *testing.T) {
	dir := t.TempDir()
	usagePath := filepath.Join(dir, "usage.json")
	require.NoError(t, os.WriteFile(usagePath, []byte(`{
		"version":1,"run_id":"run-1","call_id":"call-1","task_id":"task-1",
		"usage_status":"actual","input_tokens_total":100,"uncached_input_tokens":null,
		"cached_input_tokens":null,"cache_creation_input_tokens":null,"cache_read_input_tokens":null,
		"output_tokens_total":20,"reasoning_tokens":null,"tool_tokens":null,
		"raw_total_tokens":120,"actual_cost_usd":0.01,
		"estimated_total_tokens":null,"estimated_cost_usd":null
	}`), 0o600))

	err := runTelemetryRecord(dir, recordParams{
		specID: "SPEC-1", agent: "executor", phase: "execute", action: "agent",
		status: telemetry.StatusPass, usageJSON: usagePath,
	})
	require.NoError(t, err)

	events, err := telemetry.LoadAllEvents(dir)
	require.NoError(t, err)
	agentEvents := telemetry.FilterByType(events, telemetry.EventTypeAgentRun)
	require.Len(t, agentEvents, 1)
	var run telemetry.AgentRun
	require.NoError(t, json.Unmarshal(agentEvents[0].Data, &run))
	require.Len(t, run.Usage, 1)
	assert.Equal(t, "call-1", run.Usage[0].CallID)
	assert.Equal(t, int64(120), *run.Usage[0].RawTotalTokens)
}

func TestTelemetryReader_ReconstructsUsageAcrossShortLivedRecordBridgeCalls(t *testing.T) {
	dir := t.TempDir()
	usagePath := filepath.Join(dir, "usage.json")
	require.NoError(t, os.WriteFile(usagePath, []byte(`{
		"version":1,"run_id":"run-1","call_id":"call-1","task_id":"task-1",
		"usage_status":"actual","input_tokens_total":10,"uncached_input_tokens":null,
		"cached_input_tokens":null,"cache_creation_input_tokens":null,"cache_read_input_tokens":null,
		"output_tokens_total":2,"reasoning_tokens":null,"tool_tokens":null,
		"raw_total_tokens":12,"actual_cost_usd":0.001,
		"estimated_total_tokens":null,"estimated_cost_usd":null
	}`), 0o600))
	require.NoError(t, runTelemetryRecord(dir, recordParams{specID: "SPEC-1", action: "start", qualityMode: "ultra"}))
	require.NoError(t, runTelemetryRecord(dir, recordParams{
		specID: "SPEC-1", agent: "executor", phase: "execute", action: "agent",
		status: telemetry.StatusPass, acceptanceStatus: telemetry.StatusPass, usageJSON: usagePath,
	}))
	require.NoError(t, runTelemetryRecord(dir, recordParams{specID: "SPEC-1", action: "end", status: telemetry.StatusPass}))

	run, err := telemetry.LatestPipelineRun(dir)
	require.NoError(t, err)
	require.NotNil(t, run)
	require.Len(t, run.Phases, 1)
	require.Len(t, run.Phases[0].Agents, 1)
	assert.Equal(t, "call-1", run.Phases[0].Agents[0].CallID)
}

func TestRunTelemetryRecord_AgentUsageJSONRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	usagePath := filepath.Join(dir, "usage.json")
	require.NoError(t, os.WriteFile(usagePath, []byte(`{"version":1,"run_id":"r","call_id":"c","usage_status":"actual","unknown":1}`), 0o600))

	err := runTelemetryRecord(dir, recordParams{
		specID: "SPEC-1", agent: "executor", action: "agent", usageJSON: usagePath,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field")
}
