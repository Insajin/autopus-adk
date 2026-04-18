package orchestra

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunDebate_SurfacesFailedProviders verifies that runDebate returns
// the list of failed providers from Round 1 (Phase 1) instead of silently dropping them.
// Reproduces bug: debate.go:17 used `round1Responses, _, err := runParallel(...)`,
// discarding the `failed` slice and leaving OrchestraResult.FailedProviders empty.
func TestRunDebate_SurfacesFailedProviders(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("gemini"),        // good
			emptyOutputProvider("claude"), // empty output → failed
			badArgsProvider("codex"),      // exit non-zero → failed
		},
		Strategy:       StrategyDebate,
		Prompt:         "topic",
		TimeoutSeconds: 10,
	}

	responses, _, failed, err := runDebate(context.Background(), cfg)
	require.NoError(t, err)
	assert.Len(t, responses, 1, "only gemini succeeded")
	assert.Len(t, failed, 2, "claude (empty) and codex (exit non-zero) must be in failed")

	failedNames := make([]string, len(failed))
	for i, f := range failed {
		failedNames[i] = f.Name
	}
	assert.Contains(t, failedNames, "claude")
	assert.Contains(t, failedNames, "codex")
}

// TestRunOrchestra_Debate_PopulatesFailedProviders verifies that the public
// RunOrchestra API surfaces failed debate providers in OrchestraResult.FailedProviders.
func TestRunOrchestra_Debate_PopulatesFailedProviders(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("gemini"),
			emptyOutputProvider("claude"),
		},
		Strategy:       StrategyDebate,
		Prompt:         "topic",
		TimeoutSeconds: 10,
	}

	result, err := RunOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Responses, 1, "only gemini appears in successful responses")
	assert.Len(t, result.FailedProviders, 1, "claude must surface in FailedProviders")
	assert.Equal(t, "claude", result.FailedProviders[0].Name)
	assert.Contains(t, result.FailedProviders[0].Error, "empty output")
}

// TestRunDebate_RebuttalFailures_Surface verifies that failures in Round 2
// (rebuttal round) are also captured, not silently dropped.
// Two good providers both succeed across R1 and R2 → no failures expected.
// The negative assertion guarantees the new aggregation path does not inject
// spurious failures when the rebuttal round is healthy.
func TestRunDebate_RebuttalFailures_Surface(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("good-1"),
			echoProvider("good-2"),
		},
		Strategy:       StrategyDebate,
		Prompt:         "topic",
		TimeoutSeconds: 10,
		DebateRounds:   2,
	}

	responses, roundHistory, failed, err := runDebate(context.Background(), cfg)
	require.NoError(t, err)
	assert.Len(t, responses, 2, "both providers succeed in final round")
	assert.Len(t, roundHistory, 2, "2 rounds executed")
	assert.Empty(t, failed, "no failures expected with 2 good providers")
}

// TestRunRebuttalRound_EmptyOutputProviderSurfaced tests the rebuttal-specific
// failure filtering directly. An empty-output provider in rebuttal must land
// in the failed slice rather than the responses slice.
func TestRunRebuttalRound_EmptyOutputProviderSurfaced(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("good"),
			emptyOutputProvider("silent"),
		},
		Strategy:       StrategyDebate,
		Prompt:         "topic",
		TimeoutSeconds: 10,
	}
	prev := []ProviderResponse{
		{Provider: "good", Output: "r1 output"},
		{Provider: "silent", Output: "r1 fake output"},
	}

	responses, failed, err := runRebuttalRound(context.Background(), cfg, prev)
	require.NoError(t, err)
	assert.Len(t, responses, 1, "only echoProvider succeeds in rebuttal")
	assert.Len(t, failed, 1, "silent (empty output) must surface as a rebuttal failure")
	assert.Equal(t, "silent", failed[0].Name)
	assert.Contains(t, failed[0].Error, "empty output")
}
