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
// Prices are in USD per 1M tokens as of March 2026.
func DefaultPricingTable() map[string]ModelPricing {
	return map[string]ModelPricing{
		"claude-opus-4": {
			InputPricePerMillion:  15.0,
			OutputPricePerMillion: 75.0,
		},
		"claude-sonnet-4": {
			InputPricePerMillion:  3.0,
			OutputPricePerMillion: 15.0,
		},
		"claude-haiku-4.5": {
			InputPricePerMillion:  0.80,
			OutputPricePerMillion: 4.0,
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
			"planner":   "claude-opus-4",
			"architect": "claude-opus-4",
			"executor":  "claude-opus-4",
			"tester":    "claude-opus-4",
			"reviewer":  "claude-opus-4",
			"validator": "claude-opus-4",
		}
	case "balanced":
		// Strategic agents use opus; execution agents use sonnet; validation uses haiku.
		return map[string]string{
			"planner":   "claude-opus-4",
			"architect": "claude-opus-4",
			"executor":  "claude-sonnet-4",
			"tester":    "claude-sonnet-4",
			"reviewer":  "claude-sonnet-4",
			"validator": "claude-haiku-4.5",
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
