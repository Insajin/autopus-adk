package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMayStopDebateEarly_PreservesTwoRoundMinimum(t *testing.T) {
	t.Parallel()

	responses := []ProviderResponse{
		{Provider: "claude", Output: "1. shared claim"},
		{Provider: "codex", Output: "1. shared claim"},
	}
	cfg := OrchestraConfig{ConsensusThreshold: 0.67}

	assert.False(t, mayStopDebateEarly(1, 2, responses, cfg))
	assert.False(t, mayStopDebateEarly(1, 3, responses, cfg))
	assert.True(t, mayStopDebateEarly(2, 3, responses, cfg))
	assert.False(t, mayStopDebateEarly(3, 3, responses, cfg))
}
