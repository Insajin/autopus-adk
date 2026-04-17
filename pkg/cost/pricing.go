// Package cost provides model token pricing and cost estimation utilities.
package cost

// ModelPricing holds per-token pricing for a single model.
// Prices are expressed per one million tokens (USD).
type ModelPricing struct {
	// InputPricePerMillion is the cost in USD per 1M input tokens.
	InputPricePerMillion float64 `json:"input_price_per_million"`
	// OutputPricePerMillion is the cost in USD per 1M output tokens.
	OutputPricePerMillion float64 `json:"output_price_per_million"`
}

// DefaultPricingTable returns the canonical pricing table for supported models.
// Prices are in USD per 1M tokens as of April 2026 (Opus 4.7 launch).
// Source: https://www.anthropic.com/news/claude-opus-4-7
func DefaultPricingTable() map[string]ModelPricing {
	return map[string]ModelPricing{
		"claude-opus-4-7": {
			InputPricePerMillion:  5.0,
			OutputPricePerMillion: 25.0,
		},
		"claude-sonnet-4-6": {
			InputPricePerMillion:  3.0,
			OutputPricePerMillion: 15.0,
		},
		"claude-haiku-4-5": {
			InputPricePerMillion:  1.0,
			OutputPricePerMillion: 5.0,
		},
	}
}

// QualityModeToModels returns a map of agent name to model name for the given quality mode.
// Supported quality modes: "ultra", "balanced".
// Returns nil for unknown quality modes.
func QualityModeToModels(qualityMode string) map[string]string {
	switch qualityMode {
	case "ultra":
		// All agents use the highest-capability model.
		return map[string]string{
			"planner":   "claude-opus-4-7",
			"architect": "claude-opus-4-7",
			"executor":  "claude-opus-4-7",
			"tester":    "claude-opus-4-7",
			"reviewer":  "claude-opus-4-7",
			"validator": "claude-opus-4-7",
		}
	case "balanced":
		// Strategic agents use opus; execution and validation agents use sonnet.
		return map[string]string{
			"planner":   "claude-opus-4-7",
			"architect": "claude-opus-4-7",
			"executor":  "claude-sonnet-4-6",
			"tester":    "claude-sonnet-4-6",
			"reviewer":  "claude-sonnet-4-6",
			"validator": "claude-sonnet-4-6",
		}
	default:
		return nil
	}
}

// ModelForAgent returns the model name assigned to the given agent in the specified quality mode.
// Returns an empty string when the quality mode is unknown or the agent has no assignment.
func ModelForAgent(qualityMode, agentName string) string {
	assignments := QualityModeToModels(qualityMode)
	if assignments == nil {
		return ""
	}
	return assignments[agentName]
}
