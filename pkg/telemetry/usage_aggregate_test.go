package telemetry_test

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
)

func TestAggregateUsage_DuplicateIdentity_DeduplicatesOrBlocksOnConflict(t *testing.T) {
	c1 := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "r1", CallID: "c1", InputTokensTotal: int64Ptr(1000),
		OutputTokensTotal: int64Ptr(300), Source: telemetry.UsageSourceProvider,
	})
	c2 := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "r1", CallID: "c2", UncachedInputTokens: int64Ptr(600),
		CacheCreationInputTokens: int64Ptr(100), CacheReadInputTokens: int64Ptr(300),
		OutputTokensTotal: int64Ptr(200), Source: telemetry.UsageSourceProvider,
	})
	c3 := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "r1", CallID: "c3", UncachedInputTokens: int64Ptr(600),
		CacheCreationInputTokens: int64Ptr(100), CacheReadInputTokens: int64Ptr(300),
		OutputTokensTotal: int64Ptr(200), Source: telemetry.UsageSourceProvider,
	})
	conflictingC1 := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "r1", CallID: "c1", InputTokensTotal: int64Ptr(1000),
		OutputTokensTotal: int64Ptr(301), Source: telemetry.UsageSourceProvider,
	})

	tests := []struct {
		name         string
		inputs       []telemetry.UsageEnvelope
		wantCalls    int
		wantRawTotal *int64
		wantStatus   string
		wantReason   string
		wantBlocked  bool
	}{
		{name: "event and result copies count once", inputs: []telemetry.UsageEnvelope{c1, c1, c2}, wantCalls: 2, wantRawTotal: int64Ptr(2500), wantStatus: telemetry.UsageStatusActual},
		{name: "retry with distinct call identity counts separately", inputs: []telemetry.UsageEnvelope{c1, c1, c2, c3}, wantCalls: 3, wantRawTotal: int64Ptr(3700), wantStatus: telemetry.UsageStatusActual},
		{name: "same identity with different actual component blocks promotion", inputs: []telemetry.UsageEnvelope{c1, conflictingC1, c2}, wantCalls: 2, wantStatus: telemetry.UsageStatusUnavailable, wantReason: telemetry.UsageReasonDuplicateCallConflict, wantBlocked: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := telemetry.AggregateUsage(tt.inputs)
			assert.Equal(t, tt.wantCalls, got.UniqueModelCallCount)
			assert.Equal(t, tt.wantRawTotal, got.RawTotalTokens)
			assert.Equal(t, tt.wantStatus, got.UsageStatus)
			assert.Equal(t, tt.wantReason, got.UnavailableReason)
			assert.Equal(t, tt.wantBlocked, got.PromotionBlocked)
		})
	}
}
