package telemetry_test

import (
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeUsage_OpenAIInclusiveComponents_DoesNotDoubleCountSubsets(t *testing.T) {
	input := telemetry.UsageInput{
		RunID:             "r1",
		CallID:            "c1",
		Provider:          "openai",
		InputTokensTotal:  int64Ptr(1000),
		CachedInputTokens: int64Ptr(400),
		OutputTokensTotal: int64Ptr(300),
		ReasoningTokens:   int64Ptr(100),
		ReasoningRelation: telemetry.ComponentSubsetOfOutput,
		Source:            telemetry.UsageSourceProvider,
	}

	got := telemetry.NormalizeUsage(input)

	require.NotNil(t, got.InputTokensTotal)
	require.NotNil(t, got.UncachedInputTokens)
	require.NotNil(t, got.CachedInputTokens)
	require.NotNil(t, got.OutputTokensTotal)
	require.NotNil(t, got.ReasoningTokens)
	require.NotNil(t, got.RawTotalTokens)
	assert.EqualValues(t, 1000, *got.InputTokensTotal)
	assert.EqualValues(t, 600, *got.UncachedInputTokens)
	assert.EqualValues(t, 400, *got.CachedInputTokens)
	assert.EqualValues(t, 300, *got.OutputTokensTotal)
	assert.EqualValues(t, 100, *got.ReasoningTokens)
	assert.EqualValues(t, 1300, *got.RawTotalTokens)
	assert.Equal(t, telemetry.ComponentSubsetOfOutput, got.ReasoningRelation)
	assert.Equal(t, telemetry.UsageStatusActual, got.UsageStatus)
}

func TestNormalizeUsage_AnthropicCacheBreakdown_BuildsInclusiveInput(t *testing.T) {
	input := telemetry.UsageInput{
		RunID:                    "r1",
		CallID:                   "c2",
		Provider:                 "anthropic",
		UncachedInputTokens:      int64Ptr(600),
		CacheCreationInputTokens: int64Ptr(100),
		CacheReadInputTokens:     int64Ptr(300),
		OutputTokensTotal:        int64Ptr(200),
		Source:                   telemetry.UsageSourceProvider,
	}

	got := telemetry.NormalizeUsage(input)

	require.NotNil(t, got.InputTokensTotal)
	require.NotNil(t, got.RawTotalTokens)
	assert.EqualValues(t, 1000, *got.InputTokensTotal)
	assert.EqualValues(t, 1200, *got.RawTotalTokens)
	assert.Equal(t, int64Ptr(100), got.CacheCreationInputTokens)
	assert.Equal(t, int64Ptr(300), got.CacheReadInputTokens)
	assert.Equal(t, telemetry.UsageStatusActual, got.UsageStatus)
}

func TestNormalizeUsage_ObservedComparisonProvenance_PreservesJSON(t *testing.T) {
	t.Parallel()
	input, output := int64(10), int64(5)
	got := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "run", CallID: "call", Provider: "claude", Model: "opus", Effort: "max",
		ProviderVersion: "2.1.154", ModelVersion: "2026-07-01", RiskPolicy: "risk-v1",
		CacheStratum: "cold", ConfigHash: "config-hash", Source: telemetry.UsageSourceProvider,
		InputTokensTotal: &input, OutputTokensTotal: &output,
	})

	payload := marshalJSONObject(t, got)
	assert.Equal(t, "2.1.154", payload["provider_version"])
	assert.Equal(t, "2026-07-01", payload["model_version"])
	assert.Equal(t, "risk-v1", payload["risk_policy"])
	assert.Equal(t, "cold", payload["cache_stratum"])
	assert.Equal(t, "config-hash", payload["config_hash"])
	require.NoError(t, telemetry.ValidateUsageEnvelope(got))
}

func TestNormalizeUsage_NullableStatuses_SerializeMissingActualsAsNull(t *testing.T) {
	tests := []struct {
		name       string
		input      telemetry.UsageInput
		wantStatus string
		wantReason string
		assertJSON func(t *testing.T, payload map[string]any)
	}{
		{
			name: "cost only remains distinct from token usage",
			input: telemetry.UsageInput{
				RunID:         "r1",
				CallID:        "cost-only",
				ActualCostUSD: float64Ptr(0.04),
				Source:        telemetry.UsageSourceProvider,
			},
			wantStatus: telemetry.UsageStatusCostOnly,
			assertJSON: func(t *testing.T, payload map[string]any) {
				assert.Equal(t, 0.04, payload["actual_cost_usd"])
				assertJSONNull(t, payload, "raw_total_tokens")
				assertJSONNull(t, payload, "input_tokens_total")
				assertJSONNull(t, payload, "output_tokens_total")
			},
		},
		{
			name: "estimate is not promoted to actual",
			input: telemetry.UsageInput{
				RunID:                "r1",
				CallID:               "estimate",
				EstimatedTotalTokens: int64Ptr(1200),
				Source:               telemetry.UsageSourceEstimate,
			},
			wantStatus: telemetry.UsageStatusEstimated,
			assertJSON: func(t *testing.T, payload map[string]any) {
				assert.EqualValues(t, 1200, payload["estimated_total_tokens"])
				assertJSONNull(t, payload, "raw_total_tokens")
			},
		},
		{
			name: "absent provider usage is unavailable",
			input: telemetry.UsageInput{
				RunID:  "r1",
				CallID: "absent",
				Source: telemetry.UsageSourceProvider,
			},
			wantStatus: telemetry.UsageStatusUnavailable,
			wantReason: telemetry.UsageReasonProviderAbsent,
			assertJSON: func(t *testing.T, payload map[string]any) {
				assertJSONNull(t, payload, "raw_total_tokens")
				assertJSONNull(t, payload, "actual_cost_usd")
			},
		},
		{
			name: "unknown component relation invalidates actual total",
			input: telemetry.UsageInput{
				RunID:             "r1",
				CallID:            "ambiguous",
				InputTokensTotal:  int64Ptr(1000),
				OutputTokensTotal: int64Ptr(300),
				ReasoningTokens:   int64Ptr(100),
				Source:            telemetry.UsageSourceProvider,
			},
			wantStatus: telemetry.UsageStatusUnavailable,
			wantReason: telemetry.UsageReasonComponentRelationUnknown,
			assertJSON: func(t *testing.T, payload map[string]any) {
				assertJSONNull(t, payload, "raw_total_tokens")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			envelope := telemetry.NormalizeUsage(tt.input)
			payload := marshalJSONObject(t, envelope)

			assert.Equal(t, tt.wantStatus, envelope.UsageStatus)
			assert.Equal(t, tt.wantStatus, payload["usage_status"])
			assert.Equal(t, tt.wantReason, envelope.UnavailableReason)
			if tt.wantReason != "" {
				assert.Equal(t, tt.wantReason, payload["unavailable_reason"])
			}
			tt.assertJSON(t, payload)
		})
	}
}

func TestNormalizeUsage_SeparateReasoning_AddsReasoningExactlyOnce(t *testing.T) {
	tests := []struct {
		name         string
		relation     string
		wantRawTotal *int64
		wantStatus   string
		wantReason   string
	}{
		{
			name:         "reasoning subset stays inside output total",
			relation:     telemetry.ComponentSubsetOfOutput,
			wantRawTotal: int64Ptr(1300),
			wantStatus:   telemetry.UsageStatusActual,
		},
		{
			name:         "separate reasoning is added once",
			relation:     telemetry.ComponentSeparate,
			wantRawTotal: int64Ptr(1400),
			wantStatus:   telemetry.UsageStatusActual,
		},
		{
			name:       "unknown reasoning relation cannot be guessed",
			relation:   "",
			wantStatus: telemetry.UsageStatusUnavailable,
			wantReason: telemetry.UsageReasonComponentRelationUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := telemetry.UsageInput{
				RunID:             "r-reasoning",
				CallID:            tt.name,
				InputTokensTotal:  int64Ptr(1000),
				OutputTokensTotal: int64Ptr(300),
				ReasoningTokens:   int64Ptr(100),
				ReasoningRelation: tt.relation,
				Source:            telemetry.UsageSourceProvider,
			}

			got := telemetry.NormalizeUsage(input)

			assert.Equal(t, tt.wantRawTotal, got.RawTotalTokens)
			assert.Equal(t, tt.wantStatus, got.UsageStatus)
			assert.Equal(t, tt.wantReason, got.UnavailableReason)
		})
	}
}

func marshalJSONObject(t *testing.T, value any) map[string]any {
	t.Helper()

	body, err := json.Marshal(value)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	return payload
}

func assertJSONNull(t *testing.T, payload map[string]any, key string) {
	t.Helper()

	value, exists := payload[key]
	require.True(t, exists, "JSON field %q must be present so unavailable is explicit", key)
	assert.Nil(t, value, "JSON field %q must be null, not zero or omitted", key)
}

func int64Ptr(value int64) *int64 {
	return &value
}

func float64Ptr(value float64) *float64 {
	return &value
}
