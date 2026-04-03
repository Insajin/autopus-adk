package budget

import "fmt"

// IterationBudget defines the total iteration budget and thresholds.
// @NOTE: Threshold percentages: 70% warning, 90% danger, 100% hard limit.
type IterationBudget struct {
	Limit            int
	WarnThreshold    float64 // default 0.70
	DangerThreshold  float64 // default 0.90
}

// ThresholdLevel indicates the current budget consumption level.
type ThresholdLevel int

const (
	LevelOK     ThresholdLevel = iota // below 70%
	LevelWarn                         // 70%-89%
	LevelDanger                       // 90%-99%
	LevelExhausted                    // 100%+
)

// DefaultBudget returns an IterationBudget with standard thresholds.
func DefaultBudget(limit int) IterationBudget {
	return IterationBudget{
		Limit:           limit,
		WarnThreshold:   0.70,
		DangerThreshold: 0.90,
	}
}

// Evaluate returns the ThresholdLevel for the given count against this budget.
func (b IterationBudget) Evaluate(count int) ThresholdLevel {
	if b.Limit <= 0 {
		return LevelOK
	}
	ratio := float64(count) / float64(b.Limit)
	switch {
	case ratio >= 1.0:
		return LevelExhausted
	case ratio >= b.DangerThreshold:
		return LevelDanger
	case ratio >= b.WarnThreshold:
		return LevelWarn
	default:
		return LevelOK
	}
}

// String returns a human-readable representation.
func (b IterationBudget) String() string {
	return fmt.Sprintf("Budget{limit=%d, warn=%.0f%%, danger=%.0f%%}",
		b.Limit, b.WarnThreshold*100, b.DangerThreshold*100)
}
