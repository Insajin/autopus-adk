package telemetry_test

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

func TestValidateUsageEnvelope_ValidStatusesPass(t *testing.T) {
	t.Parallel()

	actual := validationActualUsage()
	costOnly := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "run", CallID: "cost", Source: telemetry.UsageSourceProvider,
		ActualCostUSD: validationFloat64(0.01),
	})
	estimated := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "run", CallID: "estimate", Source: telemetry.UsageSourceEstimate,
		EstimatedTotalTokens: validationInt64(50),
	})
	unavailable := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "run", CallID: "absent", Source: telemetry.UsageSourceProvider,
	})

	for _, envelope := range []telemetry.UsageEnvelope{actual, costOnly, estimated, unavailable} {
		if err := telemetry.ValidateUsageEnvelope(envelope); err != nil {
			t.Fatalf("valid %q envelope rejected: %v", envelope.UsageStatus, err)
		}
	}
}

func TestValidateUsageEnvelope_InvalidIdentityVersionAndStatusFail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mutate     func(*telemetry.UsageEnvelope)
		wantReason string
	}{
		{name: "version", mutate: func(v *telemetry.UsageEnvelope) { v.Version++ }, wantReason: "unsupported version"},
		{name: "run identity", mutate: func(v *telemetry.UsageEnvelope) { v.RunID = "" }, wantReason: "run_id and call_id"},
		{name: "call identity", mutate: func(v *telemetry.UsageEnvelope) { v.CallID = "" }, wantReason: "run_id and call_id"},
		{name: "status enum", mutate: func(v *telemetry.UsageEnvelope) { v.UsageStatus = "measured" }, wantReason: "invalid usage_status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := validationActualUsage()
			tt.mutate(&envelope)
			assertValidationError(t, envelope, tt.wantReason)
		})
	}
}

func TestValidateUsageEnvelope_NegativeTokenAndCostFieldsFail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*telemetry.UsageEnvelope)
	}{
		{name: "raw total", mutate: func(v *telemetry.UsageEnvelope) { v.RawTotalTokens = validationInt64(-1) }},
		{name: "estimated total", mutate: func(v *telemetry.UsageEnvelope) { v.EstimatedTotalTokens = validationInt64(-1) }},
		{name: "actual cost", mutate: func(v *telemetry.UsageEnvelope) { v.ActualCostUSD = validationFloat64(-0.01) }},
		{name: "estimated cost", mutate: func(v *telemetry.UsageEnvelope) { v.EstimatedCostUSD = validationFloat64(-0.01) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := validationActualUsage()
			tt.mutate(&envelope)
			assertValidationError(t, envelope, "must be non-negative")
		})
	}
}

func TestValidateUsageEnvelope_InvalidActualComponentsFail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mutate     func(*telemetry.UsageEnvelope)
		wantReason string
	}{
		{name: "missing input", mutate: func(v *telemetry.UsageEnvelope) { v.InputTokensTotal = nil }, wantReason: "requires inclusive input"},
		{name: "missing output", mutate: func(v *telemetry.UsageEnvelope) { v.OutputTokensTotal = nil }, wantReason: "requires inclusive input"},
		{name: "missing raw", mutate: func(v *telemetry.UsageEnvelope) { v.RawTotalTokens = nil }, wantReason: "requires inclusive input"},
		{name: "wrong raw", mutate: func(v *telemetry.UsageEnvelope) { v.RawTotalTokens = validationInt64(999) }, wantReason: "component totals are inconsistent"},
		{name: "cache components", mutate: func(v *telemetry.UsageEnvelope) {
			v.CachedInputTokens = validationInt64(2)
			v.UncachedInputTokens = validationInt64(9)
		}, wantReason: "component totals are inconsistent"},
		{name: "reasoning relation enum", mutate: func(v *telemetry.UsageEnvelope) {
			v.ReasoningTokens = validationInt64(1)
			v.ReasoningRelation = "unknown"
		}, wantReason: "component totals are inconsistent"},
		{name: "tool relation enum", mutate: func(v *telemetry.UsageEnvelope) {
			v.ToolTokens = validationInt64(1)
			v.ToolRelation = "unknown"
		}, wantReason: "component totals are inconsistent"},
		{name: "unavailable reason", mutate: func(v *telemetry.UsageEnvelope) {
			v.UnavailableReason = telemetry.UsageReasonProviderAbsent
		}, wantReason: "cannot have unavailable_reason"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := validationActualUsage()
			tt.mutate(&envelope)
			assertValidationError(t, envelope, tt.wantReason)
		})
	}
}

func TestValidateUsageEnvelope_InvalidNonActualShapesFail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		envelope   telemetry.UsageEnvelope
		wantReason string
	}{
		{name: "cost only without cost", envelope: validationEnvelope(telemetry.UsageStatusCostOnly), wantReason: "cost_only requires"},
		{name: "cost only with raw", envelope: func() telemetry.UsageEnvelope {
			v := validationEnvelope(telemetry.UsageStatusCostOnly)
			v.ActualCostUSD = validationFloat64(0.01)
			v.RawTotalTokens = validationInt64(1)
			return v
		}(), wantReason: "cost_only requires"},
		{name: "cost only with actual component", envelope: func() telemetry.UsageEnvelope {
			v := validationEnvelope(telemetry.UsageStatusCostOnly)
			v.ActualCostUSD = validationFloat64(0.01)
			v.InputTokensTotal = validationInt64(1)
			return v
		}(), wantReason: "cost_only requires"},
		{name: "estimated without estimate", envelope: validationEnvelope(telemetry.UsageStatusEstimated), wantReason: "estimated usage requires"},
		{name: "estimated with raw", envelope: func() telemetry.UsageEnvelope {
			v := validationEnvelope(telemetry.UsageStatusEstimated)
			v.EstimatedTotalTokens = validationInt64(1)
			v.RawTotalTokens = validationInt64(1)
			return v
		}(), wantReason: "estimated usage requires"},
		{name: "estimated with actual component", envelope: func() telemetry.UsageEnvelope {
			v := validationEnvelope(telemetry.UsageStatusEstimated)
			v.EstimatedTotalTokens = validationInt64(1)
			v.OutputTokensTotal = validationInt64(1)
			return v
		}(), wantReason: "estimated usage requires"},
		{name: "unavailable with raw", envelope: func() telemetry.UsageEnvelope {
			v := validationEnvelope(telemetry.UsageStatusUnavailable)
			v.RawTotalTokens = validationInt64(1)
			v.UnavailableReason = telemetry.UsageReasonProviderAbsent
			return v
		}(), wantReason: "unavailable usage requires"},
		{name: "unavailable without reason", envelope: validationEnvelope(telemetry.UsageStatusUnavailable), wantReason: "unavailable usage requires"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidationError(t, tt.envelope, tt.wantReason)
		})
	}
}

func validationActualUsage() telemetry.UsageEnvelope {
	return telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "run", CallID: "call", Source: telemetry.UsageSourceProvider,
		InputTokensTotal: validationInt64(10), OutputTokensTotal: validationInt64(5),
	})
}

func validationEnvelope(status string) telemetry.UsageEnvelope {
	return telemetry.UsageEnvelope{
		Version: telemetry.UsageEnvelopeVersion, RunID: "run", CallID: "call", UsageStatus: status,
	}
}

func assertValidationError(t *testing.T, envelope telemetry.UsageEnvelope, want string) {
	t.Helper()
	err := telemetry.ValidateUsageEnvelope(envelope)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("ValidateUsageEnvelope() error = %v, want substring %q", err, want)
	}
}

func validationInt64(value int64) *int64       { return &value }
func validationFloat64(value float64) *float64 { return &value }
