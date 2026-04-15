package orchestra

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunInteractiveDebate_AllEmptyProviders_ReturnsError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	cfg := OrchestraConfig{
		Strategy:     StrategyDebate,
		DebateRounds: 1,
		Prompt:       "all empty debate",
		Providers: []ProviderConfig{
			emptyOutputProvider("claude"),
			emptyOutputProvider("codex"),
		},
		TimeoutSeconds: 5,
		Terminal:       nil,
	}

	result, err := runInteractiveDebate(context.Background(), cfg)

	require.Error(t, err, "blank debate artifacts must not be treated as success")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "fallback failed")
	assert.Contains(t, err.Error(), "empty output")
}
