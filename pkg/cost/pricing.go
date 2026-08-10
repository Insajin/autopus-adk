// Package cost provides model token pricing and cost estimation utilities.
package cost

import (
	"strings"
	"sync"

	"github.com/insajin/autopus-adk/pkg/config"
)

// ModelPricing holds per-token pricing for a single model.
// Prices are expressed per one million tokens (USD).
type ModelPricing struct {
	// InputPricePerMillion is the cost in USD per 1M input tokens.
	InputPricePerMillion float64 `json:"input_price_per_million"`
	// OutputPricePerMillion is the cost in USD per 1M output tokens.
	OutputPricePerMillion float64 `json:"output_price_per_million"`
}

// DefaultPricingTable returns the canonical pricing table for supported models.
// Prices are in USD per 1M tokens. Opus 5 is the current default model;
// Opus 4.8 and 4.7 pricing is retained because both remain selectable.
// Opus 5 pricing verified against official docs (checked 2026-07-25):
// $5 input / $25 output per MTok, identical to Opus 4.8 and 4.7.
// Source: https://platform.claude.com/docs/en/about-claude/models/overview
func DefaultPricingTable() map[string]ModelPricing {
	return map[string]ModelPricing{
		config.ClaudeOpusModel: {
			InputPricePerMillion:  5.0,
			OutputPricePerMillion: 25.0,
		},
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
		config.ClaudeSonnetModel: {
			InputPricePerMillion:  3.0,
			OutputPricePerMillion: 15.0,
		},
		config.ClaudeHaikuModel: {
			InputPricePerMillion:  1.0,
			OutputPricePerMillion: 5.0,
		},
		// @AX:NOTE [AUTO]: Fable pricing is keyed only by the resolved full model ID; dynamic aliases remain intentionally unpriced. @AX:SPEC SPEC-FABLE5-001
		"claude-fable-5": {
			InputPricePerMillion:  10.0,
			OutputPricePerMillion: 50.0,
		},
	}
}

// defaultQuality snapshots the shipped quality presets once. config.Quality is
// the single tier source; cost keeps no second tier table of its own.
var defaultQuality = sync.OnceValue(func() config.QualityConf {
	return config.DefaultFullConfig("").Quality
})

// workflowRoleAliases maps workflow phase roles that are not canonical source
// agents onto the canonical agent whose tier they inherit. pkg/cost owns this
// table because the phase vocabulary is a workflow concern rather than a tier
// source. test_scaffold is route_team's test-scaffolding phase, so it tracks
// the tester tier; it must resolve to a non-empty model because the generated
// workflow JS whitelist rejects an empty one (SPEC-HARNESS-WORKFLOW-TEAM-001).
var workflowRoleAliases = map[string]string{
	"test_scaffold": "tester",
}

// QualityModeToModels returns a map of agent name to Claude model slug for the
// given quality mode. Tiers are derived from the default config's quality
// presets, so a preset promotion reaches cost accounting without a second
// table. Keys cover every canonical agent, the underscore spelling used by
// workflow phase ids, and the workflow-only roles above.
// Returns nil for unknown quality modes.
func QualityModeToModels(qualityMode string) map[string]string {
	quality := defaultQuality()
	if _, ok := quality.Presets[qualityMode]; !ok {
		return nil
	}
	view := quality.WithGlobalOverride(qualityMode)

	names := config.CanonicalAgentNames()
	models := make(map[string]string, 2*len(names)+len(workflowRoleAliases))
	for _, name := range names {
		models[name] = view.ClaudeAgentModel(name, "")
		// Workflow schemas spell multi-word roles with underscores.
		// config.NormalizeAgentName folds that spelling back onto the agent
		// file name, so both keys resolve to the same tier.
		if alias := strings.ReplaceAll(name, "-", "_"); alias != name {
			models[alias] = view.ClaudeAgentModel(alias, "")
		}
	}
	for alias, canonical := range workflowRoleAliases {
		models[alias] = view.ClaudeAgentModel(canonical, "")
	}
	return models
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
