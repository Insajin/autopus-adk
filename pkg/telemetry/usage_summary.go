package telemetry

import "fmt"

// ValidateUsageEnvelope rejects malformed normalized receipts without
// converting missing values to zero or estimates to actual usage.
func ValidateUsageEnvelope(envelope UsageEnvelope) error {
	if envelope.Version != UsageEnvelopeVersion {
		return fmt.Errorf("usage envelope: unsupported version %d", envelope.Version)
	}
	if envelope.RunID == "" || envelope.CallID == "" {
		return fmt.Errorf("usage envelope: run_id and call_id are required")
	}
	switch envelope.UsageStatus {
	case UsageStatusActual, UsageStatusCostOnly, UsageStatusEstimated, UsageStatusUnavailable:
	default:
		return fmt.Errorf("usage envelope: invalid usage_status %q", envelope.UsageStatus)
	}
	if negativeInt64(envelope.RawTotalTokens) || negativeInt64(envelope.EstimatedTotalTokens) ||
		negativeFloat64(envelope.ActualCostUSD) || negativeFloat64(envelope.EstimatedCostUSD) {
		return fmt.Errorf("usage envelope: token and cost values must be non-negative")
	}
	if envelope.UsageStatus == UsageStatusActual {
		if envelope.InputTokensTotal == nil || envelope.OutputTokensTotal == nil || envelope.RawTotalTokens == nil {
			return fmt.Errorf("usage envelope: actual usage requires inclusive input, output, and raw totals")
		}
		if envelope.UnavailableReason != "" {
			return fmt.Errorf("usage envelope: actual usage cannot have unavailable_reason")
		}
		normalized := NormalizeUsage(UsageInput{
			RunID: envelope.RunID, CallID: envelope.CallID, TaskID: envelope.TaskID,
			Attempt: envelope.Attempt, Provider: envelope.Provider, Model: envelope.Model,
			Effort: envelope.Effort, Phase: envelope.Phase, Role: envelope.Role,
			ProviderVersion: envelope.ProviderVersion, ModelVersion: envelope.ModelVersion,
			RiskPolicy: envelope.RiskPolicy, CacheStratum: envelope.CacheStratum, ConfigHash: envelope.ConfigHash,
			Source: envelope.UsageSource, SourceSchema: envelope.SourceSchema,
			InputTokensTotal: envelope.InputTokensTotal, UncachedInputTokens: envelope.UncachedInputTokens,
			CachedInputTokens:        envelope.CachedInputTokens,
			CacheCreationInputTokens: envelope.CacheCreationInputTokens,
			CacheReadInputTokens:     envelope.CacheReadInputTokens, OutputTokensTotal: envelope.OutputTokensTotal,
			ReasoningTokens: envelope.ReasoningTokens, ReasoningRelation: envelope.ReasoningRelation,
			ToolTokens: envelope.ToolTokens, ToolRelation: envelope.ToolRelation,
			ActualCostUSD: envelope.ActualCostUSD, EstimatedTotalTokens: envelope.EstimatedTotalTokens,
			EstimatedCostUSD: envelope.EstimatedCostUSD, PricingVersion: envelope.PricingVersion,
		})
		if normalized.UsageStatus != UsageStatusActual || !sameActualUsage(normalized, envelope) {
			return fmt.Errorf("usage envelope: actual component totals are inconsistent")
		}
	}
	if envelope.UsageStatus == UsageStatusCostOnly {
		if envelope.ActualCostUSD == nil || envelope.RawTotalTokens != nil || hasAnyActualTokens(envelope) {
			return fmt.Errorf("usage envelope: cost_only requires actual cost and null actual tokens")
		}
	}
	if envelope.UsageStatus == UsageStatusEstimated {
		if envelope.EstimatedTotalTokens == nil || envelope.RawTotalTokens != nil || hasAnyActualTokens(envelope) {
			return fmt.Errorf("usage envelope: estimated usage requires estimate and null actual tokens")
		}
	}
	if envelope.UsageStatus == UsageStatusUnavailable {
		if envelope.RawTotalTokens != nil || envelope.UnavailableReason == "" {
			return fmt.Errorf("usage envelope: unavailable usage requires null raw total and reason")
		}
	}
	return nil
}

func negativeInt64(value *int64) bool     { return value != nil && *value < 0 }
func negativeFloat64(value *float64) bool { return value != nil && *value < 0 }
