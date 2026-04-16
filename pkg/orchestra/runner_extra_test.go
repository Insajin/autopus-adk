package orchestra

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunDebate_WithRebuttalRound verifies that DebateRounds >= 2 triggers rebuttal.
func TestRunDebate_WithRebuttalRound(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("debater1"),
			echoProvider("debater2"),
		},
		Strategy:       StrategyDebate,
		Prompt:         "debate with rebuttal",
		TimeoutSeconds: 10,
		DebateRounds:   2,
	}

	responses, roundHistory, err := runDebate(context.Background(), cfg)
	require.NoError(t, err)
	// After rebuttal, responses should contain updated responses from both debaters.
	// Round history should have Round 1 + Round 2 entries.
	assert.Len(t, roundHistory, 2, "debate with 2 rounds should produce 2 round history entries")
	assert.NotEmpty(t, responses)
}

// TestRunRebuttalRound_Basic verifies that runRebuttalRound produces one response per provider.
func TestRunRebuttalRound_Basic(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("a"),
			echoProvider("b"),
		},
		Prompt:         "topic",
		TimeoutSeconds: 10,
	}

	prevResponses := []ProviderResponse{
		{Provider: "a", Output: "response from a"},
		{Provider: "b", Output: "response from b"},
	}

	responses, err := runRebuttalRound(context.Background(), cfg, prevResponses)
	require.NoError(t, err)
	assert.Len(t, responses, 2)
}

// TestRunRebuttalRound_AllFail verifies error when all rebuttal providers fail.
func TestRunRebuttalRound_AllFail(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			failProvider("bad1"),
			failProvider("bad2"),
		},
		Prompt:         "topic",
		TimeoutSeconds: 10,
	}

	prevResponses := []ProviderResponse{
		{Provider: "bad1", Output: "previous"},
		{Provider: "bad2", Output: "previous"},
	}

	_, err := runRebuttalRound(context.Background(), cfg, prevResponses)
	assert.Error(t, err)
}

// TestRunProvider_PromptViaArgs verifies that PromptViaArgs=true uses args-based prompt delivery.
func TestRunProvider_PromptViaArgs(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	// Use echo as a provider that accepts args (PromptViaArgs=true)
	p := ProviderConfig{
		Name:          "echo-via-args",
		Binary:        "echo",
		Args:          []string{},
		PromptViaArgs: true,
	}

	resp, err := runProvider(context.Background(), p, "hello from args")
	require.NoError(t, err)
	assert.Contains(t, resp.Output, "hello from args")
}

// TestRunProvider_StdinBased verifies that PromptViaArgs=false uses stdin-based delivery.
func TestRunProvider_StdinBased(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	// cat reads from stdin and echoes it back (PromptViaArgs=false)
	p := ProviderConfig{
		Name:          "cat-stdin",
		Binary:        "cat",
		Args:          []string{},
		PromptViaArgs: false,
	}

	resp, err := runProvider(context.Background(), p, "stdin content test")
	require.NoError(t, err)
	assert.Contains(t, resp.Output, "stdin content test")
}
