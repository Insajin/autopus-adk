package telemetry_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentRun_JSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	run := telemetry.AgentRun{
		AgentName:       "executor",
		StartTime:       now,
		EndTime:         now.Add(5 * time.Second),
		Duration:        5 * time.Second,
		Status:          telemetry.StatusPass,
		FilesModified:   3,
		EstimatedTokens: 1200,
	}

	data, err := json.Marshal(run)
	require.NoError(t, err)

	var got telemetry.AgentRun
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, run.AgentName, got.AgentName)
	assert.Equal(t, run.Status, got.Status)
	assert.Equal(t, run.FilesModified, got.FilesModified)
	assert.Equal(t, run.EstimatedTokens, got.EstimatedTokens)
	assert.Equal(t, run.Duration, got.Duration)
}

func TestPipelineRun_JSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	pipeline := telemetry.PipelineRun{
		SpecID:        "SPEC-TELE-001",
		StartTime:     now,
		EndTime:       now.Add(30 * time.Second),
		TotalDuration: 30 * time.Second,
		Phases: []telemetry.PhaseRecord{
			{
				Name:      "RED",
				StartTime: now,
				EndTime:   now.Add(10 * time.Second),
				Duration:  10 * time.Second,
				Status:    telemetry.StatusPass,
				Agents: []telemetry.AgentRun{
					{AgentName: "tester", Status: telemetry.StatusPass},
				},
			},
		},
		RetryCount:  1,
		FinalStatus: telemetry.StatusPass,
		QualityMode: "strict",
	}

	data, err := json.Marshal(pipeline)
	require.NoError(t, err)

	var got telemetry.PipelineRun
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, pipeline.SpecID, got.SpecID)
	assert.Equal(t, pipeline.FinalStatus, got.FinalStatus)
	assert.Equal(t, pipeline.RetryCount, got.RetryCount)
	assert.Equal(t, pipeline.QualityMode, got.QualityMode)
	require.Len(t, got.Phases, 1)
	assert.Equal(t, "RED", got.Phases[0].Name)
	require.Len(t, got.Phases[0].Agents, 1)
	assert.Equal(t, "tester", got.Phases[0].Agents[0].AgentName)
}

func TestEvent_JSONRoundTrip(t *testing.T) {
	run := telemetry.AgentRun{
		AgentName: "executor",
		Status:    telemetry.StatusFail,
	}
	payload, err := json.Marshal(run)
	require.NoError(t, err)

	event := telemetry.Event{
		Type:      telemetry.EventTypeAgentRun,
		Timestamp: time.Now().UTC().Truncate(time.Millisecond),
		Data:      payload,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var got telemetry.Event
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, telemetry.EventTypeAgentRun, got.Type)
	assert.NotEmpty(t, got.Data)
}

func TestStatusConstants(t *testing.T) {
	assert.Equal(t, "PASS", telemetry.StatusPass)
	assert.Equal(t, "FAIL", telemetry.StatusFail)
}

func TestEventTypeConstants(t *testing.T) {
	assert.Equal(t, "agent_run", telemetry.EventTypeAgentRun)
	assert.Equal(t, "phase_start", telemetry.EventTypePhaseStart)
	assert.Equal(t, "phase_end", telemetry.EventTypePhaseEnd)
	assert.Equal(t, "pipeline_start", telemetry.EventTypePipelineStart)
	assert.Equal(t, "pipeline_end", telemetry.EventTypePipelineEnd)
}

// mockCostEstimator verifies the CostEstimator interface is implementable.
type mockCostEstimator struct{}

func (m *mockCostEstimator) EstimateCost(run telemetry.AgentRun) float64 {
	return float64(run.EstimatedTokens) * 0.000002
}

func TestCostEstimatorInterface(t *testing.T) {
	var estimator telemetry.CostEstimator = &mockCostEstimator{}
	run := telemetry.AgentRun{EstimatedTokens: 1000}
	cost := estimator.EstimateCost(run)
	assert.InDelta(t, 0.002, cost, 1e-9)
}
