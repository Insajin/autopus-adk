package routing

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
			"claude": {Simple: "claude-sonnet-4-6", Medium: "claude-sonnet-4-6", Complex: "claude-opus-4-6"},
			"codex":  {Simple: "gpt-4o-mini", Medium: "gpt-4o", Complex: "o3"},
			"gemini": {Simple: "gemini-2.0-flash", Medium: "gemini-2.5-pro", Complex: "gemini-2.5-pro"},
		},
	}
}
