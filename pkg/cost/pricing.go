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
// Prices are in USD per 1M tokens. Opus 4.8 is the current default model;
// Opus 4.7 pricing is retained because it remains a valid selectable model.
// Opus 4.8 pricing verified against official docs (checked 2026-05-29):
// $5 input / $25 output per MTok, identical to Opus 4.7.
// Source: https://platform.claude.com/docs/en/about-claude/models/overview
func DefaultPricingTable() map[string]ModelPricing {
	return map[string]ModelPricing{
		"claude-opus-4-8": {
			InputPricePerMillion:  5.0,
			OutputPricePerMillion: 25.0,
		},
		"claude-opus-4-7": {
			InputPricePerMillion:  5.0,
			OutputPricePerMillion: 25.0,
		},
		"claude-sonnet-4-6": {
			InputPricePerMillion:  3.0,
			OutputPricePerMillion: 15.0,
		},
		// Sonnet 5 standard pricing (intro pricing is $2/$10 through 2026-08-31);
		// standard pricing is retained here as the durable value. Checked 2026-07-13.
		"claude-sonnet-5": {
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
			"planner":          "claude-opus-4-8",
			"architect":        "claude-opus-4-8",
			"executor":         "claude-opus-4-8",
			"tester":           "claude-opus-4-8",
			"reviewer":         "claude-opus-4-8",
			"validator":        "claude-opus-4-8",
			"test_scaffold":    "claude-opus-4-8",
			"annotator":        "claude-opus-4-8",
			"security_auditor": "claude-opus-4-8",
		}
	case "balanced":
		// Strategic agents use opus; execution and validation agents use sonnet-5.
		// Team-phase roles (test_scaffold, annotator, security_auditor) are
		// execution/validation-class and therefore mapped to sonnet-5 in balanced mode.
		return map[string]string{
			"planner":          "claude-opus-4-8",
			"architect":        "claude-opus-4-8",
			"executor":         "claude-sonnet-5",
			"tester":           "claude-sonnet-5",
			"reviewer":         "claude-sonnet-5",
			"validator":        "claude-sonnet-5",
			"test_scaffold":    "claude-sonnet-5",
			"annotator":        "claude-sonnet-5",
			"security_auditor": "claude-sonnet-5",
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
