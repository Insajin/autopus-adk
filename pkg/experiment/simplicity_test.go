package experiment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateSimplicity_AutoDiscard(t *testing.T) {
	t.Parallel()

	// Improvement is negative (metric got worse for minimize direction)
	score := CalculateSimplicity(
		100.0,  // baseline
		105.0,  // current (worse for minimize)
		10,     // linesAdded
		0,      // linesRemoved
		Minimize,
	)

	// When metric gets worse, simplicity score should indicate auto-discard
	assert.Less(t, score, 0.0, "worse metric should yield negative simplicity score")
}

func TestCalculateSimplicity_AutoKeep(t *testing.T) {
	t.Parallel()

	// Large improvement with few lines changed -> very high simplicity score
	score := CalculateSimplicity(
		100.0, // baseline
		50.0,  // current (50% improvement for minimize)
		2,     // linesAdded (very few)
		1,     // linesRemoved
		Minimize,
	)

	assert.Greater(t, score, 0.0, "better metric should yield positive simplicity score")
}

func TestCalculateSimplicity_Normal(t *testing.T) {
	t.Parallel()

	// Moderate improvement with moderate lines changed
	score := CalculateSimplicity(
		100.0, // baseline
		95.0,  // current (5% improvement)
		50,    // linesAdded
		20,    // linesRemoved
		Minimize,
	)

	// Should be a small positive score (5% improvement normalized by lines)
	assert.Greater(t, score, 0.0, "slight improvement should yield positive simplicity score")
}

func TestCalculateSimplicity_MaximizeDirection(t *testing.T) {
	t.Parallel()

	// For maximize, higher current is better
	score := CalculateSimplicity(
		100.0, // baseline
		120.0, // current (20% improvement for maximize)
		10,    // linesAdded
		5,     // linesRemoved
		Maximize,
	)

	assert.Greater(t, score, 0.0, "maximize direction: improvement should be positive")
}

func TestCalculateSimplicity_ZeroLines(t *testing.T) {
	t.Parallel()

	// Zero lines changed with improvement: should handle without division by zero
	score := CalculateSimplicity(
		100.0,
		90.0,
		0,
		0,
		Minimize,
	)

	// Should not panic; score should be positive (improvement with no lines)
	assert.Greater(t, score, 0.0, "zero lines with improvement: should still return positive score")
}

func TestCalculateSimplicity_BaselineZero(t *testing.T) {
	t.Parallel()

	// Baseline is zero — should handle without division by zero
	score := CalculateSimplicity(
		0.0,
		1.0,
		5,
		0,
		Minimize,
	)

	// Should not panic; result can be any float but must not panic
	_ = score
}

func TestCalculateSimplicity_NoChange(t *testing.T) {
	t.Parallel()

	score := CalculateSimplicity(
		100.0,
		100.0,
		10,
		5,
		Minimize,
	)

	// No metric change = zero improvement
	assert.Equal(t, 0.0, score, "identical metrics should yield zero simplicity score")
}
