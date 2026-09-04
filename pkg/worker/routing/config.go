package routing

import "github.com/insajin/autopus-adk/pkg/config"

// Complexity levels for message classification.
type Complexity string

const (
	ComplexitySimple  Complexity = "simple"
	ComplexityMedium  Complexity = "medium"
	ComplexityComplex Complexity = "complex"
)

// ClassifierThresholds defines the boundaries for complexity classification.
type ClassifierThresholds struct {
	SimpleMaxChars  int // messages shorter than this are simple candidates (default: 200)
	ComplexMinChars int // messages longer than this are complex candidates (default: 1000)
}

// ProviderModels maps complexity levels to model names for a single provider.
type ProviderModels struct {
	Simple  string
	Medium  string
	Complex string
}

// RoutingConfig holds the full routing configuration.
type RoutingConfig struct {
	Enabled    bool // false by default — preserves existing behavior (REQ-ROUTE-06)
	Thresholds ClassifierThresholds
	Models     map[string]ProviderModels // provider name -> model mapping
}

// DefaultConfig returns a RoutingConfig with sensible defaults.
// Enabled is false by default (REQ-ROUTE-06).
func DefaultConfig() RoutingConfig {
	return RoutingConfig{
		Enabled: false,
		Thresholds: ClassifierThresholds{
			SimpleMaxChars:  200,
			ComplexMinChars: 1000,
		},
		Models: map[string]ProviderModels{
			"claude": {
				Simple:  config.ClaudeSonnetModel,
				Medium:  config.ClaudeOpusModel,
				Complex: config.ClaudeFableModel,
			},
			"codex": {
				Simple:  config.CodexLunaModel,
				Medium:  config.CodexSolModel,
				Complex: config.CodexAstraModel,
			},
			"gemini": {Simple: "gemini-3.8-flash", Medium: "gemini-3.1-pro", Complex: "gemini-3.1-pro"},
		},
	}
}
