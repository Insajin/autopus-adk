package telemetry_test

import (
	"fmt"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
)

func TestSummarizeUsageCoverage_ThresholdFixtures(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		actual     int
		wantPct    float64
		wantActual int
	}{
		{name: "nineteen of twenty", actual: 19, wantPct: 95, wantActual: 19},
		{name: "eighteen of twenty", actual: 18, wantPct: 90, wantActual: 18},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := telemetry.SummarizeUsageCoverage(coverageFixtures(20, tc.actual))

			assert.Equal(t, 20, got.EligibleCallCount)
			assert.Equal(t, tc.wantActual, got.ActualCompleteCallCount)
			assert.InDelta(t, tc.wantPct, got.ActualUsageCapturePct, 0.001)
			assert.False(t, got.PromotionBlocked)
		})
	}
}

func TestSummarizeUsageCoverage_DeduplicatesIdenticalCallIdentity(t *testing.T) {
	t.Parallel()

	actual := coverageActual("run-1", "call-1")
	got := telemetry.SummarizeUsageCoverage([]telemetry.UsageEnvelope{actual, actual})

	assert.Equal(t, 1, got.EligibleCallCount)
	assert.Equal(t, 1, got.ActualCompleteCallCount)
	assert.InDelta(t, 100, got.ActualUsageCapturePct, 0.001)
	assert.False(t, got.PromotionBlocked)
}

func TestSummarizeUsageCoverage_ConflictingDuplicateFailsClosed(t *testing.T) {
	t.Parallel()

	left := coverageActual("run-1", "call-1")
	right := coverageActualWithTotal("run-1", "call-1", 200)
	got := telemetry.SummarizeUsageCoverage([]telemetry.UsageEnvelope{left, right})

	assert.True(t, got.PromotionBlocked)
	assert.Equal(t, telemetry.UsageReasonDuplicateCallConflict, got.UnavailableReason)
}

func coverageFixtures(total, actual int) []telemetry.UsageEnvelope {
	out := make([]telemetry.UsageEnvelope, 0, total)
	for i := 0; i < total; i++ {
		runID := fmt.Sprintf("run-%02d", i)
		callID := fmt.Sprintf("call-%02d", i)
		if i < actual {
			out = append(out, coverageActual(runID, callID))
			continue
		}
		out = append(out, telemetry.NormalizeUsage(telemetry.UsageInput{
			RunID: runID, CallID: callID, Source: telemetry.UsageSourceProvider,
		}))
	}
	return out
}

func coverageActual(runID, callID string) telemetry.UsageEnvelope {
	return coverageActualWithTotal(runID, callID, 100)
}

func coverageActualWithTotal(runID, callID string, total int64) telemetry.UsageEnvelope {
	output := int64(0)
	return telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: runID, CallID: callID, Source: telemetry.UsageSourceProvider,
		InputTokensTotal: &total, OutputTokensTotal: &output,
	})
}
