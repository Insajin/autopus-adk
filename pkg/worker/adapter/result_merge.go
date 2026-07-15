package adapter

import "github.com/insajin/autopus-adk/pkg/telemetry"

// MergeSequentialResult preserves multiline plain-text output from providers
// that do not emit structured result envelopes.
func MergeSequentialResult(providerName string, previous TaskResult, hasPrevious bool, next TaskResult) TaskResult {
	if !hasPrevious {
		return next
	}
	previousUsage := append([]telemetry.UsageEnvelope(nil), previous.Usage...)
	incomingUsage := append([]telemetry.UsageEnvelope(nil), next.Usage...)
	toolCalls := previous.ToolCalls + next.ToolCalls
	if providerName == "gemini" && hasPrevious && isPlainTextResult(previous) && isPlainTextResult(next) {
		if previous.Output == "" {
			previous.Output = next.Output
			previous.Usage = mergeUsage(previousUsage, incomingUsage)
			previous.ToolCalls = toolCalls
			return previous
		}
		if next.Output != "" {
			previous.Output += "\n" + next.Output
		}
		previous.Usage = mergeUsage(previousUsage, incomingUsage)
		previous.ToolCalls = toolCalls
		return previous
	}
	if resultPayloadEmpty(next) {
		next = previous
	}
	if next.Output == "" {
		next.Output = previous.Output
	}
	next.Usage = mergeUsage(previousUsage, incomingUsage)
	next.ToolCalls = toolCalls
	return next
}

func resultPayloadEmpty(result TaskResult) bool {
	return result.CostUSD == 0 &&
		result.DurationMS == 0 &&
		result.SessionID == "" &&
		result.Output == "" &&
		!result.IsError &&
		result.Error == "" &&
		len(result.Artifacts) == 0
}

func isPlainTextResult(result TaskResult) bool {
	return result.CostUSD == 0 &&
		result.DurationMS == 0 &&
		result.SessionID == "" &&
		!result.IsError &&
		result.Error == "" &&
		len(result.Artifacts) == 0
}

func mergeUsage(previous, next []telemetry.UsageEnvelope) []telemetry.UsageEnvelope {
	merged := append([]telemetry.UsageEnvelope(nil), previous...)
	for _, candidate := range next {
		if candidate.RunID == "" || candidate.CallID == "" {
			merged = append(merged, candidate)
			continue
		}
		duplicate := false
		for _, existing := range merged {
			if existing.RunID != candidate.RunID || existing.CallID != candidate.CallID {
				continue
			}
			if !telemetry.AggregateUsage([]telemetry.UsageEnvelope{existing, candidate}).PromotionBlocked {
				duplicate = true
			}
			break
		}
		if !duplicate {
			merged = append(merged, candidate)
		}
	}
	return merged
}
