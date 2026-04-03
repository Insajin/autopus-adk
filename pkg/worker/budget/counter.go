package budget

import "sync"

// Counter tracks tool call invocations against an IterationBudget.
// Thread-safe for concurrent use across stream parsing goroutines.
type Counter struct {
	mu     sync.Mutex
	budget IterationBudget
	count  int
	prev   ThresholdLevel
}

// NewCounter creates a Counter for the given budget.
func NewCounter(b IterationBudget) *Counter {
	return &Counter{
		budget: b,
		prev:   LevelOK,
	}
}

// IncrementResult holds the outcome of a single tool call increment.
type IncrementResult struct {
	Count    int
	Level    ThresholdLevel
	Changed  bool // true when the threshold level changed
	Budget   IterationBudget
}

// Increment records one tool call and returns the updated state.
func (c *Counter) Increment() IncrementResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count++
	level := c.budget.Evaluate(c.count)
	changed := level != c.prev
	c.prev = level

	return IncrementResult{
		Count:   c.count,
		Level:   level,
		Changed: changed,
		Budget:  c.budget,
	}
}

// Count returns the current tool call count.
func (c *Counter) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

// Remaining returns how many tool calls are left before the hard limit.
func (c *Counter) Remaining() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	rem := c.budget.Limit - c.count
	if rem < 0 {
		return 0
	}
	return rem
}

// Reset sets the counter to zero and adjusts the budget limit.
// Used when carrying over unused budget between pipeline phases.
func (c *Counter) Reset(newLimit int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count = 0
	c.prev = LevelOK
	c.budget.Limit = newLimit
}
