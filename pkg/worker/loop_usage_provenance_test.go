package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBindUsageIdentity_SupervisorProvenanceOverridesProviderOutput(t *testing.T) {
	t.Parallel()
	task := trustedUsageTaskConfig()

	bound := bindUsageIdentity([]telemetry.UsageEnvelope{spoofedUsageEnvelope()}, task, "claude")

	require.Len(t, bound, 1)
	assert.Equal(t, "claude", bound[0].Provider)
	assertTrustedUsageProvenance(t, bound[0], task)
}

func TestHandleTask_ProvenancePayloadReachesTaskConfigAndUsage(t *testing.T) {
	t.Parallel()
	spoofed := spoofedUsageEnvelope()
	mock := &mockAdapter{
		name:   "mock",
		script: `echo '{"type":"result","output":"done"}'`,
		parseFn: func(line []byte) (adapter.StreamEvent, error) {
			return adapter.StreamEvent{Type: "result", Data: append([]byte(nil), line...), Usage: []telemetry.UsageEnvelope{spoofed}}, nil
		},
		extractFn: func(event adapter.StreamEvent) adapter.TaskResult {
			return adapter.TaskResult{Output: "done", Usage: event.Usage}
		},
	}
	wl := &WorkerLoop{config: LoopConfig{Provider: mock, WorkDir: t.TempDir()}}
	payload, err := json.Marshal(taskPayloadMessage{
		Prompt: "do work", Model: "trusted-model", Effort: "trusted-effort",
		ProviderVersion: "trusted-provider-version", ModelVersion: "trusted-model-version",
		RiskPolicy: "trusted-risk", CacheStratum: "trusted-cache", ConfigHash: "trusted-config",
	})
	require.NoError(t, err)

	_, err = wl.handleTask(context.Background(), "trusted-task", payload)

	require.NoError(t, err)
	assertTrustedTaskProvenance(t, mock.last)
}

func TestPipelineExecutor_ProvenanceReachesEveryPhaseTaskAndUsage(t *testing.T) {
	t.Parallel()
	spoofed := spoofedUsageEnvelope()
	mock := &pipelineMockAdapter{
		script: `echo '{"type":"result","output":"done"}'`,
		parseFn: func(line []byte) (adapter.StreamEvent, error) {
			return adapter.StreamEvent{Type: "result", Data: append([]byte(nil), line...), Usage: []telemetry.UsageEnvelope{spoofed}}, nil
		},
		extractFn: func(event adapter.StreamEvent) adapter.TaskResult {
			return adapter.TaskResult{Output: "done", Usage: event.Usage}
		},
	}
	pe := NewPipelineExecutor(mock, "", t.TempDir())
	pe.SetUsageIdentity("trusted-run", 2, "trusted-effort", "trusted-role")
	pe.SetUsageProvenance("trusted-provider-version", "trusted-model-version", "trusted-risk", "trusted-cache", "trusted-config")

	result, err := pe.ExecuteWithPlan(context.Background(), "trusted-task", "prompt", "trusted-model", []Phase{PhasePlanner, PhaseReviewer})

	require.NoError(t, err)
	require.Len(t, mock.calls, 2)
	for _, call := range mock.calls {
		assertTrustedTaskProvenance(t, call)
	}
	require.Len(t, result.Usage, 2)
	for _, usage := range result.Usage {
		assertTrustedUsageProvenance(t, usage, trustedUsageTaskConfig())
	}
}

func trustedUsageTaskConfig() adapter.TaskConfig {
	return adapter.TaskConfig{
		TaskID: "trusted-task", RunID: "trusted-run", CallID: "trusted-call", Attempt: 2,
		Model: "trusted-model", Effort: "trusted-effort", ProviderVersion: "trusted-provider-version",
		ModelVersion: "trusted-model-version", RiskPolicy: "trusted-risk",
		CacheStratum: "trusted-cache", ConfigHash: "trusted-config",
	}
}

func spoofedUsageEnvelope() telemetry.UsageEnvelope {
	input, output := int64(10), int64(2)
	return telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "spoof-run", CallID: "spoof-call", Provider: "mock", Model: "spoof-model", Effort: "spoof-effort",
		ProviderVersion: "spoof-provider-version", ModelVersion: "spoof-model-version",
		RiskPolicy: "spoof-risk", CacheStratum: "spoof-cache", ConfigHash: "spoof-config",
		Source: telemetry.UsageSourceProvider, InputTokensTotal: &input, OutputTokensTotal: &output,
	})
}

func assertTrustedTaskProvenance(t *testing.T, task adapter.TaskConfig) {
	t.Helper()
	assert.Equal(t, "trusted-provider-version", task.ProviderVersion)
	assert.Equal(t, "trusted-model-version", task.ModelVersion)
	assert.Equal(t, "trusted-risk", task.RiskPolicy)
	assert.Equal(t, "trusted-cache", task.CacheStratum)
	assert.Equal(t, "trusted-config", task.ConfigHash)
}

func assertTrustedUsageProvenance(t *testing.T, usage telemetry.UsageEnvelope, task adapter.TaskConfig) {
	t.Helper()
	assert.Equal(t, task.Model, usage.Model)
	assert.Equal(t, task.Effort, usage.Effort)
	assert.Equal(t, task.ProviderVersion, usage.ProviderVersion)
	assert.Equal(t, task.ModelVersion, usage.ModelVersion)
	assert.Equal(t, task.RiskPolicy, usage.RiskPolicy)
	assert.Equal(t, task.CacheStratum, usage.CacheStratum)
	assert.Equal(t, task.ConfigHash, usage.ConfigHash)
}
