package telemetry_test

import (
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregateUsage_MissingIdentity_BlocksPromotion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		runID  string
		callID string
	}{
		{name: "missing run identity", callID: "c1"},
		{name: "missing call identity", runID: "r1"},
		{name: "missing both identities"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			envelope := telemetry.NormalizeUsage(telemetry.UsageInput{
				RunID: tt.runID, CallID: tt.callID,
				InputTokensTotal: edgeInt64Ptr(10), OutputTokensTotal: edgeInt64Ptr(5),
				Source: telemetry.UsageSourceProvider,
			})
			got := telemetry.AggregateUsage([]telemetry.UsageEnvelope{envelope})

			assert.Equal(t, telemetry.UsageStatusUnavailable, envelope.UsageStatus)
			assert.Equal(t, telemetry.UsageReasonIdentityMissing, envelope.UnavailableReason)
			assert.Equal(t, telemetry.UsageStatusUnavailable, got.UsageStatus)
			assert.Equal(t, telemetry.UsageReasonIdentityMissing, got.UnavailableReason)
			assert.True(t, got.PromotionBlocked)
		})
	}
}

func TestAggregateUsage_MetadataOnlyDuplicate_DoesNotConflict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*telemetry.UsageEnvelope)
	}{
		{
			name: "task differs",
			mutate: func(envelope *telemetry.UsageEnvelope) {
				envelope.TaskID = "task-2"
			},
		},
		{
			name: "phase differs",
			mutate: func(envelope *telemetry.UsageEnvelope) {
				envelope.Phase = "review"
			},
		},
		{
			name: "role differs",
			mutate: func(envelope *telemetry.UsageEnvelope) {
				envelope.Role = "reviewer"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			original := edgeActualEnvelope("r1", "c1", 100, 20)
			duplicate := original
			tt.mutate(&duplicate)

			got := telemetry.AggregateUsage([]telemetry.UsageEnvelope{original, duplicate})

			assert.Equal(t, 1, got.UniqueModelCallCount)
			assert.Equal(t, telemetry.UsageStatusActual, got.UsageStatus)
			require.NotNil(t, got.RawTotalTokens)
			assert.EqualValues(t, 120, *got.RawTotalTokens)
			assert.Empty(t, got.UnavailableReason)
			assert.False(t, got.PromotionBlocked)
		})
	}
}

func TestAggregateUsage_AggregationSemanticMismatch_ConflictsDeterministically(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*telemetry.UsageEnvelope)
	}{
		{name: "status", mutate: func(v *telemetry.UsageEnvelope) { v.UsageStatus = telemetry.UsageStatusEstimated }},
		{name: "actual cost", mutate: func(v *telemetry.UsageEnvelope) { v.ActualCostUSD = edgeFloat64Ptr(0.08) }},
		{name: "estimated tokens", mutate: func(v *telemetry.UsageEnvelope) { v.EstimatedTotalTokens = edgeInt64Ptr(999) }},
		{name: "estimated cost", mutate: func(v *telemetry.UsageEnvelope) { v.EstimatedCostUSD = edgeFloat64Ptr(0.09) }},
		{name: "unavailable reason", mutate: func(v *telemetry.UsageEnvelope) { v.UnavailableReason = "different_reason" }},
		{name: "usage source", mutate: func(v *telemetry.UsageEnvelope) { v.UsageSource = telemetry.UsageSourceEstimate }},
		{name: "source schema", mutate: func(v *telemetry.UsageEnvelope) { v.SourceSchema = "provider.usage.v2" }},
		{name: "provider", mutate: func(v *telemetry.UsageEnvelope) { v.Provider = "spoof" }},
		{name: "model", mutate: func(v *telemetry.UsageEnvelope) { v.Model = "other-model" }},
		{name: "effort", mutate: func(v *telemetry.UsageEnvelope) { v.Effort = "low" }},
		{name: "provider version", mutate: func(v *telemetry.UsageEnvelope) { v.ProviderVersion = "other-provider-version" }},
		{name: "model version", mutate: func(v *telemetry.UsageEnvelope) { v.ModelVersion = "other-model-version" }},
		{name: "risk policy", mutate: func(v *telemetry.UsageEnvelope) { v.RiskPolicy = "other-risk" }},
		{name: "cache stratum", mutate: func(v *telemetry.UsageEnvelope) { v.CacheStratum = "warm" }},
		{name: "config hash", mutate: func(v *telemetry.UsageEnvelope) { v.ConfigHash = "other-config" }},
		{name: "pricing version", mutate: func(v *telemetry.UsageEnvelope) { v.PricingVersion = "2026-08" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			original := edgeActualEnvelope("run", "call", 100, 20)
			original.Provider, original.Model, original.Effort = "claude", "opus", "max"
			original.ProviderVersion, original.ModelVersion = "2.1.154", "2026-07-01"
			original.RiskPolicy, original.CacheStratum, original.ConfigHash = "risk-v1", "cold", "config-hash"
			original.SourceSchema, original.PricingVersion = "provider.usage.v1", "2026-07"
			original.ActualCostUSD = edgeFloat64Ptr(0.04)
			duplicate := original
			tt.mutate(&duplicate)

			for _, inputs := range [][]telemetry.UsageEnvelope{{original, duplicate}, {duplicate, original}} {
				got := telemetry.AggregateUsage(inputs)
				assert.True(t, got.PromotionBlocked)
				assert.Equal(t, telemetry.UsageReasonDuplicateCallConflict, got.UnavailableReason)
				assert.Equal(t, telemetry.UsageStatusUnavailable, got.UsageStatus)
			}
		})
	}
}

func TestNormalizeUsage_InconsistentInclusiveInput_FailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input telemetry.UsageInput
	}{
		{
			name: "cached input plus uncached input differs from inclusive input",
			input: telemetry.UsageInput{
				InputTokensTotal: edgeInt64Ptr(100), UncachedInputTokens: edgeInt64Ptr(70),
				CachedInputTokens: edgeInt64Ptr(20), OutputTokensTotal: edgeInt64Ptr(10),
			},
		},
		{
			name: "split cache plus uncached input differs from inclusive input",
			input: telemetry.UsageInput{
				InputTokensTotal: edgeInt64Ptr(100), UncachedInputTokens: edgeInt64Ptr(60),
				CacheCreationInputTokens: edgeInt64Ptr(10), CacheReadInputTokens: edgeInt64Ptr(20),
				OutputTokensTotal: edgeInt64Ptr(10),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.input.RunID, tt.input.CallID = "r1", "c1"
			tt.input.Source = telemetry.UsageSourceProvider
			got := telemetry.NormalizeUsage(tt.input)

			assert.Equal(t, telemetry.UsageStatusUnavailable, got.UsageStatus)
			assert.Equal(t, telemetry.UsageReasonInconsistentComponents, got.UnavailableReason)
			assert.Nil(t, got.RawTotalTokens)
		})
	}
}

func TestNormalizeUsage_NegativeActualComponent_FailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*telemetry.UsageInput)
	}{
		{name: "inclusive input", mutate: func(v *telemetry.UsageInput) { v.InputTokensTotal = edgeInt64Ptr(-1) }},
		{name: "uncached input", mutate: func(v *telemetry.UsageInput) { v.UncachedInputTokens = edgeInt64Ptr(-1) }},
		{name: "cached input", mutate: func(v *telemetry.UsageInput) { v.CachedInputTokens = edgeInt64Ptr(-1) }},
		{name: "cache creation", mutate: func(v *telemetry.UsageInput) { v.CacheCreationInputTokens = edgeInt64Ptr(-1) }},
		{name: "cache read", mutate: func(v *telemetry.UsageInput) { v.CacheReadInputTokens = edgeInt64Ptr(-1) }},
		{name: "output", mutate: func(v *telemetry.UsageInput) { v.OutputTokensTotal = edgeInt64Ptr(-1) }},
		{name: "reasoning", mutate: func(v *telemetry.UsageInput) { v.ReasoningTokens = edgeInt64Ptr(-1) }},
		{name: "tool", mutate: func(v *telemetry.UsageInput) { v.ToolTokens = edgeInt64Ptr(-1) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := telemetry.UsageInput{
				RunID: "r1", CallID: "c1", InputTokensTotal: edgeInt64Ptr(10),
				OutputTokensTotal: edgeInt64Ptr(5), ReasoningRelation: telemetry.ComponentSeparate,
				ToolRelation: telemetry.ComponentSeparate, Source: telemetry.UsageSourceProvider,
			}
			tt.mutate(&input)
			got := telemetry.NormalizeUsage(input)

			assert.Equal(t, telemetry.UsageStatusUnavailable, got.UsageStatus)
			assert.Equal(t, telemetry.UsageReasonInconsistentComponents, got.UnavailableReason)
			assert.Nil(t, got.RawTotalTokens)
		})
	}
}

func TestAggregateUsage_InputSlice_RemainsUnchanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		inputs []telemetry.UsageEnvelope
	}{
		{
			name: "distinct and duplicate calls",
			inputs: []telemetry.UsageEnvelope{
				edgeActualEnvelope("r1", "c2", 80, 20),
				edgeActualEnvelope("r1", "c1", 100, 30),
				edgeActualEnvelope("r1", "c1", 100, 30),
			},
		},
		{
			name: "conflicting duplicate calls",
			inputs: []telemetry.UsageEnvelope{
				edgeActualEnvelope("r1", "c1", 100, 30),
				edgeActualEnvelope("r1", "c1", 100, 31),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			before, err := json.Marshal(tt.inputs)
			require.NoError(t, err)
			telemetry.AggregateUsage(tt.inputs)
			after, err := json.Marshal(tt.inputs)
			require.NoError(t, err)

			assert.JSONEq(t, string(before), string(after))
		})
	}
}

func edgeActualEnvelope(runID, callID string, input, output int64) telemetry.UsageEnvelope {
	return telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: runID, CallID: callID, InputTokensTotal: edgeInt64Ptr(input),
		OutputTokensTotal: edgeInt64Ptr(output), Source: telemetry.UsageSourceProvider,
	})
}

func edgeInt64Ptr(value int64) *int64       { return &value }
func edgeFloat64Ptr(value float64) *float64 { return &value }
