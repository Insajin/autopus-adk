package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCmuxAdapter_ImplementsTerminal verifies CmuxAdapter satisfies the Terminal interface at compile time.
func TestCmuxAdapter_ImplementsTerminal(t *testing.T) {
	t.Parallel()

	var _ Terminal = (*CmuxAdapter)(nil)
}

// TestTmuxAdapter_ImplementsTerminal verifies TmuxAdapter satisfies the Terminal interface at compile time.
func TestTmuxAdapter_ImplementsTerminal(t *testing.T) {
	t.Parallel()

	var _ Terminal = (*TmuxAdapter)(nil)
}

// TestPlainAdapter_ImplementsTerminal verifies PlainAdapter satisfies the Terminal interface at compile time.
func TestPlainAdapter_ImplementsTerminal(t *testing.T) {
	t.Parallel()

	var _ Terminal = (*PlainAdapter)(nil)
}

// TestDirection_Constants verifies Horizontal and Vertical direction constants have expected values.
func TestDirection_Constants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		direction Direction
		want      Direction
	}{
		{"Horizontal is zero value", Horizontal, Direction(0)},
		{"Vertical is non-zero", Vertical, Direction(1)},
		{"Horizontal and Vertical differ", Horizontal, Horizontal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.direction)
		})
	}

	assert.NotEqual(t, Horizontal, Vertical, "Horizontal and Vertical must be distinct")
}
