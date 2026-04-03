package budget

import (
	"sync"
	"testing"
)

// REQ-BUDGET-05: EventToolCall increments counter.

func TestCounter_Increment(t *testing.T) {
	c := NewCounter(DefaultBudget(100))

	r := c.Increment()
	if r.Count != 1 {
		t.Errorf("Count = %d, want 1", r.Count)
	}
	if r.Level != LevelOK {
		t.Errorf("Level = %d, want LevelOK", r.Level)
	}
}

func TestCounter_ThresholdTransitions(t *testing.T) {
	c := NewCounter(DefaultBudget(10))

	// Increment to 7 (70% of 10) — should transition to LevelWarn.
	for range 6 {
		c.Increment()
	}
	r := c.Increment() // count=7
	if r.Level != LevelWarn {
		t.Errorf("at count 7: Level = %d, want LevelWarn", r.Level)
	}
	if !r.Changed {
		t.Error("at count 7: expected Changed=true")
	}

	// Increment to 9 (90% of 10) — should transition to LevelDanger.
	c.Increment() // 8
	r = c.Increment() // 9
	if r.Level != LevelDanger {
		t.Errorf("at count 9: Level = %d, want LevelDanger", r.Level)
	}
	if !r.Changed {
		t.Error("at count 9: expected Changed=true")
	}

	// Increment to 10 (100%) — should transition to LevelExhausted.
	r = c.Increment() // 10
	if r.Level != LevelExhausted {
		t.Errorf("at count 10: Level = %d, want LevelExhausted", r.Level)
	}
	if !r.Changed {
		t.Error("at count 10: expected Changed=true")
	}
}

func TestCounter_NoChangeFalse(t *testing.T) {
	c := NewCounter(DefaultBudget(100))

	c.Increment() // 1 -> OK
	r := c.Increment() // 2 -> still OK
	if r.Changed {
		t.Error("expected Changed=false when level stays the same")
	}
}

func TestCounter_Remaining(t *testing.T) {
	c := NewCounter(DefaultBudget(10))
	if c.Remaining() != 10 {
		t.Errorf("Remaining = %d, want 10", c.Remaining())
	}

	for range 8 {
		c.Increment()
	}
	if c.Remaining() != 2 {
		t.Errorf("Remaining = %d, want 2", c.Remaining())
	}
}

func TestCounter_Reset(t *testing.T) {
	c := NewCounter(DefaultBudget(10))
	for range 5 {
		c.Increment()
	}

	c.Reset(20)
	if c.Count() != 0 {
		t.Errorf("Count after reset = %d, want 0", c.Count())
	}
	if c.Remaining() != 20 {
		t.Errorf("Remaining after reset = %d, want 20", c.Remaining())
	}
}

// REQ-BUDGET-05: Thread safety test.
func TestCounter_ConcurrentAccess(t *testing.T) {
	c := NewCounter(DefaultBudget(1000))
	var wg sync.WaitGroup

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				c.Increment()
			}
		}()
	}

	wg.Wait()
	if c.Count() != 1000 {
		t.Errorf("Count = %d, want 1000 after concurrent increments", c.Count())
	}
}
