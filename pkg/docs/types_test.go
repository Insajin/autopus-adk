package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTokenBudget_Calculate verifies the adaptive token budget calculation table.
// Given: varying numbers of libraries to fetch
// When: CalculateTokenBudget is called
// Then: the per-library budget matches the expected adaptive schedule
func TestTokenBudget_Calculate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		libCount  int
		minPerLib int
		maxPerLib int
		maxTotal  int
	}{
		{"1 library gets ~5000", 1, 4000, 6000, 10000},
		{"2 libraries get ~3000 each", 2, 2500, 3500, 10000},
		{"3 libraries get ~2500 each", 3, 2000, 3000, 10000},
		{"4 libraries get ~2000 each", 4, 1500, 2500, 10000},
		{"5 libraries get ~2000 each", 5, 1500, 2500, 10000},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			perLib := CalculateTokenBudget(tt.libCount)
			total := perLib * tt.libCount

			assert.GreaterOrEqual(t, perLib, tt.minPerLib,
				"per-library budget too low for %d libs", tt.libCount)
			assert.LessOrEqual(t, perLib, tt.maxPerLib,
				"per-library budget too high for %d libs", tt.libCount)
			assert.LessOrEqual(t, total, tt.maxTotal,
				"total budget exceeds hard cap for %d libs", tt.libCount)
		})
	}
}
