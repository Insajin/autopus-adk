package orchestra

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helper function tests (consensusReached, countNonEmpty, perRoundTimeout, buildDebateResult, mergeByStrategyWithRoundHistory) ---

// TestConsensusReached_Different verifies no consensus for different outputs.
func TestConsensusReached_Different(t *testing.T) {
	t.Parallel()
	responses := []ProviderResponse{
		{Provider: "claude", Output: "answer A with lots of detail"},
		{Provider: "gemini", Output: "completely different answer B"},
	}
	assert.False(t, consensusReached(responses, OrchestraConfig{}))
}

// TestConsensusReached_SingleProvider verifies single provider returns false.
func TestConsensusReached_SingleProvider(t *testing.T) {
	t.Parallel()
	assert.False(t, consensusReached([]ProviderResponse{{Provider: "claude", Output: "one"}}, OrchestraConfig{}))
}

// TestConsensusReached_EmptyOutput verifies empty outputs returns false.
func TestConsensusReached_EmptyOutput(t *testing.T) {
	t.Parallel()
	responses := []ProviderResponse{
		{Provider: "claude", Output: ""},
		{Provider: "gemini", Output: ""},
	}
	assert.False(t, consensusReached(responses, OrchestraConfig{}))
}

// TestConsensusReached_ConfigurableThreshold verifies threshold parameterization.
func TestConsensusReached_ConfigurableThreshold(t *testing.T) {
	t.Parallel()
	responses := []ProviderResponse{
		{Provider: "claude", Output: "answer A with lots of detail"},
		{Provider: "gemini", Output: "completely different answer B"},
	}

	// Default (0) -> uses 0.66
	assert.False(t, consensusReached(responses, OrchestraConfig{}))
	assert.False(t, consensusReached(responses, OrchestraConfig{ConsensusThreshold: 0}))

	// Custom 0.8 -> uses 0.8 (still no consensus with different answers)
	assert.False(t, consensusReached(responses, OrchestraConfig{ConsensusThreshold: 0.8}))

	// Single provider -- always returns false regardless of threshold
	single := []ProviderResponse{{Provider: "claude", Output: "one"}}
	assert.False(t, consensusReached(single, OrchestraConfig{ConsensusThreshold: 0.5}))
}

// TestCountNonEmpty verifies counting of non-empty responses.
func TestCountNonEmpty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		resps    []ProviderResponse
		expected int
	}{
		{"all non-empty", []ProviderResponse{{Output: "a"}, {Output: "b"}, {Output: "c"}}, 3},
		{"mixed", []ProviderResponse{{Output: "a"}, {Output: ""}, {Output: "c"}}, 2},
		{"all empty", []ProviderResponse{{Output: ""}, {Output: ""}}, 0},
		{"nil slice", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, countNonEmpty(tt.resps))
		})
	}
}

// TestPerRoundTimeout verifies per-round timeout calculation.
func TestPerRoundTimeout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		total    int
		rounds   int
		expected time.Duration
	}{
		{"120s / 3 rounds", 120, 3, 45 * time.Second},       // 40 < 45 -> floor applied
		{"60s / 1 round", 60, 1, 60 * time.Second},
		{"zero total defaults", 0, 2, 60 * time.Second},
		{"negative total defaults", -1, 4, 45 * time.Second}, // 30 < 45 -> floor applied
		{"zero rounds defaults", 60, 0, 60 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, perRoundTimeout(tt.total, tt.rounds))
		})
	}
}

// TestPerRoundTimeout_MinimumFloor verifies 45-second minimum floor per round.
func TestPerRoundTimeout_MinimumFloor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		total    int
		rounds   int
		expected time.Duration
	}{
		{"floor applied", 60, 3, 45 * time.Second},   // 60/3=20 < 45 -> 45
		{"no floor needed", 120, 2, 60 * time.Second}, // 120/2=60 > 45
		{"default total", 0, 1, 120 * time.Second},    // 120/1=120 > 45
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, perRoundTimeout(tt.total, tt.rounds))
		})
	}
}

// TestBuildDebateResult verifies result construction.
func TestBuildDebateResult(t *testing.T) {
	t.Parallel()
	responses := []ProviderResponse{
		{Provider: "claude", Output: "claude says"},
		{Provider: "gemini", Output: "gemini says"},
	}
	history := [][]ProviderResponse{responses}
	result := buildDebateResult(OrchestraConfig{Strategy: StrategyDebate}, responses, history, time.Now())
	assert.Equal(t, StrategyDebate, result.Strategy)
	assert.Len(t, result.Responses, 2)
	assert.Len(t, result.RoundHistory, 1)
	assert.NotEmpty(t, result.Merged)
}

// TestBuildDebateResult_NilResponses verifies nil responses fallback.
func TestBuildDebateResult_NilResponses(t *testing.T) {
	t.Parallel()
	result := buildDebateResult(OrchestraConfig{Strategy: StrategyDebate}, nil, nil, time.Now())
	assert.Contains(t, result.Merged, "0 rounds completed")
}

// TestBuildDebateResult_SingleRound verifies single round result.
func TestBuildDebateResult_SingleRound(t *testing.T) {
	t.Parallel()
	responses := []ProviderResponse{{Provider: "claude", Output: "only"}}
	history := [][]ProviderResponse{responses}
	result := buildDebateResult(OrchestraConfig{Strategy: StrategyDebate}, responses, history, time.Now())
	assert.Len(t, result.RoundHistory, 1)
	assert.NotEmpty(t, result.Merged)
}

// TestMergeByStrategyWithRoundHistory verifies round history merge.
func TestMergeByStrategyWithRoundHistory(t *testing.T) {
	t.Parallel()
	rounds := [][]ProviderResponse{
		{{Provider: "claude", Output: "r1"}, {Provider: "gemini", Output: "r1"}},
		{{Provider: "claude", Output: "r2"}, {Provider: "gemini", Output: "r2"}},
	}
	result := mergeByStrategyWithRoundHistory(rounds, OrchestraConfig{Strategy: StrategyDebate})
	require.NotNil(t, result)
	assert.Equal(t, StrategyDebate, result.Strategy)
	assert.Len(t, result.RoundHistory, 2)
	assert.Len(t, result.Responses, 2)
}

// TestMergeByStrategyWithRoundHistory_Empty verifies empty rounds.
func TestMergeByStrategyWithRoundHistory_Empty(t *testing.T) {
	t.Parallel()
	result := mergeByStrategyWithRoundHistory(nil, OrchestraConfig{Strategy: StrategyDebate})
	require.NotNil(t, result)
	assert.Nil(t, result.Responses)
}

// TestMergeByStrategyWithRoundHistory_SingleRound verifies single round.
func TestMergeByStrategyWithRoundHistory_SingleRound(t *testing.T) {
	t.Parallel()
	rounds := [][]ProviderResponse{{{Provider: "claude", Output: "single"}}}
	result := mergeByStrategyWithRoundHistory(rounds, OrchestraConfig{Strategy: StrategyDebate})
	assert.Len(t, result.Responses, 1)
	assert.Len(t, result.RoundHistory, 1)
}
