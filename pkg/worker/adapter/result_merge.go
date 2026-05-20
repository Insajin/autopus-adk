package adapter

// MergeSequentialResult preserves multiline plain-text output from providers
// that do not emit structured result envelopes.
func MergeSequentialResult(providerName string, previous TaskResult, hasPrevious bool, next TaskResult) TaskResult {
	if providerName == "gemini" && hasPrevious && isPlainTextResult(previous) && isPlainTextResult(next) {
		if previous.Output == "" {
			previous.Output = next.Output
			return previous
		}
		if next.Output != "" {
			previous.Output += "\n" + next.Output
		}
		return previous
	}
	return next
}

func isPlainTextResult(result TaskResult) bool {
	return result.CostUSD == 0 &&
		result.DurationMS == 0 &&
		result.SessionID == "" &&
		!result.IsError &&
		result.Error == "" &&
		len(result.Artifacts) == 0
}
