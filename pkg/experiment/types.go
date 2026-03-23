package experiment

import "time"

// Direction indicates whether lower or higher metric values are better.
type Direction int

const (
	// Minimize means lower metric values are preferred.
	Minimize Direction = iota
	// Maximize means higher metric values are preferred.
	Maximize
)

// String returns a human-readable representation of the direction.
func (d Direction) String() string {
	if d == Maximize {
		return "maximize"
	}
	return "minimize"
}

// IsBetter reports whether current is strictly better than best for this direction.
func (d Direction) IsBetter(current, best float64) bool {
	if d == Maximize {
		return current > best
	}
	return current < best
}

// Config holds all configuration for an experiment run.
type Config struct {
	MetricCmd           string
	MetricKey           string
	Direction           Direction
	TargetFiles         []string
	Scope               []string      // empty = same as TargetFiles
	MaxIterations       int           // default 50
	CircuitBreakerN     int           // default 10
	ExperimentTimeout   time.Duration // default 5m
	MetricRuns          int           // default 1
	SimplicityThreshold float64       // default 0.001 (0.1%)
	SessionID           string
}

// @AX:NOTE [AUTO]: SimplicityThreshold default 0.001 means improvement/lines ratio must be >= 0.1%
// to keep a change. Derived from SPEC-XLOOP-001 R13 simplicity gate requirement.
// DefaultConfig returns a Config with sane defaults applied.
func DefaultConfig() Config {
	return Config{
		Direction:           Minimize,
		MaxIterations:       50,
		CircuitBreakerN:     10,
		ExperimentTimeout:   5 * time.Minute,
		MetricRuns:          1,
		SimplicityThreshold: 0.001,
	}
}

// MetricOutput holds the parsed result of a metric command execution.
type MetricOutput struct {
	Metric   float64
	Unit     string
	Metadata map[string]any
}

// Result captures the outcome of a single experiment iteration.
type Result struct {
	Iteration    int
	CommitHash   string
	MetricValue  float64
	MetricKey    string
	Unit         string
	Status       string // keep, discard, crash, timeout, scope-violation
	Description  string
	LinesChanged int
	Timestamp    time.Time
}

// ExperimentSummary aggregates results across all iterations.
type ExperimentSummary struct {
	TotalIterations int
	KeepCount       int
	DiscardCount    int
	BaselineMetric  float64
	BestMetric      float64
	BestIteration   int
	Top5            []Result
}
