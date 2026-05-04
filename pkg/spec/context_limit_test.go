package spec_test

// Phase 1.5 test scaffold for SPEC-SPECREV-001 REQ-CTX-1 / REQ-CTX-4.
// These tests intentionally reference symbols that do not yet exist
// (spec.AdaptiveContextLimit). Compile failure here is the expected RED state
// and serves as a behavior contract for Phase 2 executors.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/spec"
)

// TestAdaptiveContextLimit_NoCeilingMapping covers REQ-CTX-1 mapping
// (cited 0..2 -> 500, 3..5 -> 1500, 6+ -> 3000) when no ceiling is applied.
func TestAdaptiveContextLimit_NoCeilingMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cited    int
		ceiling  int
		expected int
	}{
		{"cited=0 returns default 500", 0, 0, 500},
		{"cited=1 returns 500", 1, 0, 500},
		{"cited=2 boundary returns 500", 2, 0, 500},
		{"cited=3 enters mid bucket returns 1500", 3, 0, 1500},
		{"cited=5 boundary mid bucket returns 1500", 5, 0, 1500},
		{"cited=6 enters hard cap returns 3000", 6, 0, 3000},
		{"cited=10 stays at 3000", 10, 0, 3000},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := spec.AdaptiveContextLimit(tt.cited, tt.ceiling)
			assert.Equal(t, tt.expected, got,
				"cited=%d ceiling=%d expected=%d got=%d", tt.cited, tt.ceiling, tt.expected, got)
		})
	}
}

// TestAdaptiveContextLimit_CeilingApplied covers REQ-CTX-4: when the
// autopus.yaml ceiling is positive, the final value MUST be min(mapped, ceiling).
func TestAdaptiveContextLimit_CeilingApplied(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cited    int
		ceiling  int
		expected int
	}{
		{"ceiling=0 means no cap (cited=6 -> 3000)", 6, 0, 3000},
		{"ceiling negative is treated as no cap", 6, -1, 3000},
		{"ceiling smaller than mapped caps to ceiling", 6, 1200, 1200},
		{"ceiling larger than mapped keeps mapped", 3, 5000, 1500},
		{"ceiling equal to mapped keeps mapped", 4, 1500, 1500},
		{"ceiling caps default 500 when smaller", 0, 200, 200},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := spec.AdaptiveContextLimit(tt.cited, tt.ceiling)
			assert.Equal(t, tt.expected, got,
				"cited=%d ceiling=%d expected=%d got=%d", tt.cited, tt.ceiling, tt.expected, got)
		})
	}
}
