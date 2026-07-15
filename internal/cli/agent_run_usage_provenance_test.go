package cli

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBindAgentUsage_SupervisorProvenanceOverridesProviderOutput(t *testing.T) {
	t.Parallel()
	input, output := int64(10), int64(2)
	spoofed := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "spoof-run", CallID: "spoof-call", Provider: "spoof-provider",
		Model: "spoof-model", Effort: "spoof-effort", ProviderVersion: "spoof-provider-version",
		ModelVersion: "spoof-model-version", RiskPolicy: "spoof-risk",
		CacheStratum: "spoof-cache", ConfigHash: "spoof-config",
		Source: telemetry.UsageSourceProvider, InputTokensTotal: &input, OutputTokensTotal: &output,
	})
	task := adapter.TaskConfig{
		TaskID: "task", RunID: "run", CallID: "call", Attempt: 3,
		Model: "trusted-model", Effort: "trusted-effort", ProviderVersion: "trusted-provider-version",
		ModelVersion: "trusted-model-version", RiskPolicy: "trusted-risk",
		CacheStratum: "trusted-cache", ConfigHash: "trusted-config",
	}

	bound := bindAgentUsage([]telemetry.UsageEnvelope{spoofed}, task, "codex")

	require.Len(t, bound, 1)
	assert.Equal(t, "trusted-model", bound[0].Model)
	assert.Equal(t, "trusted-effort", bound[0].Effort)
	assert.Equal(t, "trusted-provider-version", bound[0].ProviderVersion)
	assert.Equal(t, "trusted-model-version", bound[0].ModelVersion)
	assert.Equal(t, "trusted-risk", bound[0].RiskPolicy)
	assert.Equal(t, "trusted-cache", bound[0].CacheStratum)
	assert.Equal(t, "trusted-config", bound[0].ConfigHash)
}
