package experiment

const defaultCircuitBreakerThreshold = 10

// CircuitBreaker tracks consecutive no-improvement iterations.
type CircuitBreaker struct {
	threshold             int
	consecutiveNoProgress int
}

// NewCircuitBreaker creates a breaker with the given threshold.
// If threshold is <= 0, it defaults to 10.
func NewCircuitBreaker(threshold int) *CircuitBreaker {
	if threshold <= 0 {
		threshold = defaultCircuitBreakerThreshold
	}
	return &CircuitBreaker{threshold: threshold}
}

// Record records whether the latest iteration showed improvement.
// Resets counter on improvement, increments on no improvement.
func (cb *CircuitBreaker) Record(improved bool) {
	if improved {
		cb.consecutiveNoProgress = 0
	} else {
		cb.consecutiveNoProgress++
	}
}

// IsTripped returns true when consecutive no-progress count >= threshold.
func (cb *CircuitBreaker) IsTripped() bool {
	return cb.consecutiveNoProgress >= cb.threshold
}

// ConsecutiveNoProgress returns the current no-progress counter.
func (cb *CircuitBreaker) ConsecutiveNoProgress() int {
	return cb.consecutiveNoProgress
}

// Reset resets the counter to zero.
func (cb *CircuitBreaker) Reset() {
	cb.consecutiveNoProgress = 0
}
