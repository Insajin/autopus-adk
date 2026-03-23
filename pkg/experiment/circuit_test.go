package experiment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker_NotTrippedInitially(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker(3)
	assert.False(t, cb.IsTripped(), "circuit breaker should not be tripped initially")
}

func TestCircuitBreaker_TripsAfterNConsecutiveFailures(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker(3)

	cb.Record(false) // discard
	assert.False(t, cb.IsTripped())

	cb.Record(false) // discard
	assert.False(t, cb.IsTripped())

	cb.Record(false) // discard — now at threshold
	assert.True(t, cb.IsTripped(), "should trip after 3 consecutive discards")
}

func TestCircuitBreaker_ResetOnImprovement(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker(3)

	cb.Record(false)
	cb.Record(false)
	cb.Record(false)
	require.True(t, cb.IsTripped())

	// Record a keep (improvement) — should reset
	cb.Record(true)
	assert.False(t, cb.IsTripped(), "circuit breaker should reset after improvement")
}

func TestCircuitBreaker_CounterResetsOnKeep(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker(3)

	cb.Record(false) // 1
	cb.Record(false) // 2
	cb.Record(true)  // keep — resets counter
	cb.Record(false) // 1 again
	cb.Record(false) // 2

	// Only 2 consecutive discards after the keep, should not trip
	assert.False(t, cb.IsTripped(), "only 2 consecutive discards, should not trip with threshold 3")
}

func TestCircuitBreaker_ThresholdOne(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker(1)

	cb.Record(false)
	assert.True(t, cb.IsTripped(), "should trip immediately with threshold 1")

	cb.Record(true)
	assert.False(t, cb.IsTripped(), "should reset after a keep")
}

func TestCircuitBreaker_LargeThreshold(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker(10)

	for i := 0; i < 9; i++ {
		cb.Record(false)
		assert.False(t, cb.IsTripped(), "should not trip at iteration %d", i+1)
	}

	cb.Record(false)
	assert.True(t, cb.IsTripped(), "should trip at 10th consecutive discard")
}

func TestNewCircuitBreaker_InvalidThreshold(t *testing.T) {
	t.Parallel()

	// Zero or negative threshold should use a sane default
	cb := NewCircuitBreaker(0)
	assert.NotNil(t, cb, "NewCircuitBreaker(0) should return a valid circuit breaker")

	cb2 := NewCircuitBreaker(-5)
	assert.NotNil(t, cb2, "NewCircuitBreaker(-5) should return a valid circuit breaker")
}
