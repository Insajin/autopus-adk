package orchestra

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// S1: StrategyRelay.IsValid() returns true.
func TestStrategyRelay_IsValid(t *testing.T) {
	t.Parallel()
	assert.True(t, StrategyRelay.IsValid())
}

// S2: agenticArgs returns correct flags per provider.
func TestAgenticArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		provider string
		wantArgs []string
	}{
		{"claude", []string{"--allowedTools", "Read,Grep,Bash,Glob"}},
		{"codex", []string{"--approval-mode", "full-auto", "--quiet"}},
		{"opencode", []string{"--auto"}},
		{"gemini", nil},
		{"unknown", nil},
	}

	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			t.Parallel()
			got := agenticArgs(tc.provider)
			assert.Equal(t, tc.wantArgs, got)
		})
	}
}

// S3: Sequential execution order (A->B->C) with file saving.
func TestRunRelay_SequentialOrder(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported for cat-based relay test")
	}

	cfg := &OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "a", Binary: "cat", Args: []string{}, PromptViaArgs: false},
			{Name: "b", Binary: "cat", Args: []string{}, PromptViaArgs: false},
			{Name: "c", Binary: "cat", Args: []string{}, PromptViaArgs: false},
		},
		Prompt:          "hello relay",
		TimeoutSeconds:  10,
		KeepRelayOutput: false,
	}

	responses, err := runRelay(context.Background(), cfg)
	require.NoError(t, err)
	assert.Len(t, responses, 3)

	// Verify sequential order by provider name
	assert.Equal(t, "a", responses[0].Provider)
	assert.Equal(t, "b", responses[1].Provider)
	assert.Equal(t, "c", responses[2].Provider)
}

// S4: buildRelayPrompt includes ## Previous Analysis by {provider} section.
func TestBuildRelayPrompt(t *testing.T) {
	t.Parallel()

	original := "original prompt"
	previous := []relayStageResult{
		{provider: "claude", output: "claude output here"},
		{provider: "codex", output: "codex output here"},
	}

	result := buildRelayPrompt(original, previous)

	assert.Contains(t, result, original)
	assert.Contains(t, result, "## Previous Analysis by claude")
	assert.Contains(t, result, "claude output here")
	assert.Contains(t, result, "## Previous Analysis by codex")
	assert.Contains(t, result, "codex output here")
}

func TestBuildRelayPrompt_NoPrevious(t *testing.T) {
	t.Parallel()
	result := buildRelayPrompt("my prompt", nil)
	assert.Equal(t, "my prompt", result)
}

// S5: CLI integration — strategy parsing accepts "relay".
func TestStrategyRelay_CLIParsing(t *testing.T) {
	t.Parallel()
	s := Strategy("relay")
	assert.True(t, s.IsValid())
}

// S6: Backward compatibility — existing strategies unaffected.
func TestExistingStrategies_UnaffectedByRelay(t *testing.T) {
	t.Parallel()

	strategies := []Strategy{
		StrategyConsensus,
		StrategyPipeline,
		StrategyDebate,
		StrategyFastest,
	}

	for _, s := range strategies {
		t.Run(string(s), func(t *testing.T) {
			t.Parallel()
			assert.True(t, s.IsValid())
		})
	}
}

// S7: cleanupRelayDir removes directory when keep=false.
func TestCleanupRelayDir_Remove(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported for cleanup test")
	}

	dir, err := os.MkdirTemp("", "autopus-relay-test-*")
	require.NoError(t, err)

	cleanupRelayDir(dir, false)

	_, statErr := os.Stat(dir)
	assert.True(t, os.IsNotExist(statErr), "directory should be removed when keep=false")
}

// S8: cleanupRelayDir preserves directory when keep=true.
func TestCleanupRelayDir_Keep(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported for keep test")
	}

	dir, err := os.MkdirTemp("", "autopus-relay-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	cleanupRelayDir(dir, true)

	_, statErr := os.Stat(dir)
	assert.NoError(t, statErr, "directory should be preserved when keep=true")
}

// S9: Partial failure handling — skip failed provider, continue.
func TestRunRelay_PartialFailure(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported for failure test")
	}

	cfg := &OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "cat", Binary: "cat", Args: []string{}, PromptViaArgs: false},
			{Name: "bad", Binary: "binary_that_does_not_exist_xyz", Args: []string{}, PromptViaArgs: false},
			{Name: "cat2", Binary: "cat", Args: []string{}, PromptViaArgs: false},
		},
		Prompt:         "partial failure test",
		TimeoutSeconds: 10,
	}

	responses, err := runRelay(context.Background(), cfg)
	// runRelay returns no error when at least one provider succeeds (skip-continue, REQ-3a)
	require.NoError(t, err)

	// All 3 providers should have response entries
	assert.Len(t, responses, 3)
	assert.Equal(t, "cat", responses[0].Provider)
	assert.Equal(t, "bad", responses[1].Provider)
	assert.Equal(t, "cat2", responses[2].Provider)

	// First provider succeeded
	assert.NotEqual(t, -1, responses[0].ExitCode)
	// Second provider was skipped (failed)
	assert.Contains(t, responses[1].Output, "[SKIPPED:")
	// Third provider continued and succeeded (skip-continue behavior)
	assert.NotEqual(t, -1, responses[2].ExitCode)
}

// S10: FormatRelay output format.
func TestFormatRelay(t *testing.T) {
	t.Parallel()

	responses := []ProviderResponse{
		{Provider: "claude", Output: "claude analysis"},
		{Provider: "codex", Output: "codex analysis"},
	}

	result := FormatRelay(responses)

	assert.Contains(t, result, "## Relay Stage 1: (by claude)")
	assert.Contains(t, result, "claude analysis")
	assert.Contains(t, result, "## Relay Stage 2: (by codex)")
	assert.Contains(t, result, "codex analysis")
}
