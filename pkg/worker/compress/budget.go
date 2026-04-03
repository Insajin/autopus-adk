package compress

// ModelWindows maps provider names to their context window sizes in tokens.
var ModelWindows = map[string]int{
	"claude":  200000,
	"codex":   128000,
	"gemini":  1000000,
	"default": 128000,
}

// SummaryMaxTokens is the hard cap for summary size.
const SummaryMaxTokens = 12288

// SummaryRatio is the fraction of window used for summary (5%).
const SummaryRatio = 0.05

// EstimateTokens returns an approximate token count.
// Heuristic: ~4 characters per token for English text.
func EstimateTokens(text string) int {
	return len(text) / 4
}

// WindowSize returns the context window for the given provider.
// Falls back to "default" if the provider is not recognized.
func WindowSize(provider string) int {
	if w, ok := ModelWindows[provider]; ok {
		return w
	}
	return ModelWindows["default"]
}

// SummaryBudget calculates the max summary size in tokens for a provider.
// It is 5% of the window, capped at SummaryMaxTokens.
func SummaryBudget(provider string) int {
	limit := int(float64(WindowSize(provider)) * SummaryRatio)
	if limit > SummaryMaxTokens {
		return SummaryMaxTokens
	}
	return limit
}

// ShouldCompress returns true when the cumulative output exceeds
// the compression threshold (50% of the model window).
func ShouldCompress(text string, provider string) bool {
	tokens := EstimateTokens(text)
	threshold := WindowSize(provider) / 2
	return tokens > threshold
}
