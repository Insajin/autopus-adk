package orchestra

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emptyOutputProvider returns a ProviderConfig that exits 0 but produces no stdout.
// This reproduces the bug where a provider succeeds but returns empty output.
func emptyOutputProvider(name string) ProviderConfig {
	if runtime.GOOS == "windows" {
		return ProviderConfig{
			Name:          name,
			Binary:        "cmd",
			Args:          []string{"/c", "exit 0"},
			PromptViaArgs: true,
		}
	}
	// 'true' command exits 0 and produces no output.
	return ProviderConfig{
		Name:          name,
		Binary:        "/usr/bin/true",
		Args:          []string{},
		PromptViaArgs: true,
	}
}

// badArgsProvider simulates codex --quiet: a real binary but with invalid flags.
// Reproduces: codex args=[--quiet] causing "unexpected argument" error.
func badArgsProvider(name string) ProviderConfig {
	if runtime.GOOS == "windows" {
		return ProviderConfig{
			Name:          name,
			Binary:        "cmd",
			Args:          []string{"/c", "exit 1"},
			PromptViaArgs: true,
		}
	}
	// Use 'false' as a stand-in for an invalid-flag scenario (exits non-zero, no output).
	return ProviderConfig{
		Name:          name,
		Binary:        "/usr/bin/false",
		Args:          []string{},
		PromptViaArgs: true,
	}
}

// TestRunProvider_EmptyOutput_IsAnError verifies that a provider returning empty
// stdout with exit code 0 is treated as a failed response, not a success.
// Reproduces bug: claude/codex returning empty output was silently accepted.
func TestRunProvider_EmptyOutput_IsAnError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	p := emptyOutputProvider("true-cmd")
	resp, err := runProvider(context.Background(), p, "some prompt")

	// After fix: empty output should be flagged via EmptyOutput field or returned as error.
	// Before fix: err == nil and resp.Output == "" (bug: silently empty).
	require.NoError(t, err, "runProvider itself should not error for exit-0 empty output")
	assert.Empty(t, resp.Output, "output is empty — this is the reproduction condition")
	// The fix: EmptyOutput flag must be set so callers can detect the silent failure.
	assert.True(t, resp.EmptyOutput, "EmptyOutput must be true when stdout is empty")
}

// TestRunParallel_EmptyOutputProviders_AreReportedAsFailed verifies that
// providers returning empty output are collected as failed, not as successful responses.
// Reproduces: claude + codex returning empty → debate missing 2/3 participants.
func TestRunParallel_EmptyOutputProviders_AreReportedAsFailed(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("gemini"),        // good: returns output
			emptyOutputProvider("claude"), // bug: empty output, exit 0
			badArgsProvider("codex"),      // bug: invalid flags, exit non-zero
		},
		Strategy:       StrategyDebate,
		Prompt:         "brainstorm topic",
		TimeoutSeconds: 10,
	}

	responses, failed, err := runParallel(context.Background(), cfg)

	// After fix: only gemini succeeds; claude + codex are in failed list.
	require.NoError(t, err, "at least one provider succeeded, so no top-level error")
	assert.Len(t, responses, 1, "only gemini should appear in successful responses")
	assert.Len(t, failed, 2, "claude (empty output) and codex (bad args) should be in failed")

	failedNames := make([]string, len(failed))
	for i, f := range failed {
		failedNames[i] = f.Name
	}
	assert.Contains(t, failedNames, "claude")
	assert.Contains(t, failedNames, "codex")
}

func TestRunParallel_AllEmptyOutputProviders_ReturnsError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			emptyOutputProvider("claude"),
			emptyOutputProvider("codex"),
		},
		Strategy:       StrategyDebate,
		Prompt:         "brainstorm topic",
		TimeoutSeconds: 10,
	}

	responses, failed, err := runParallel(context.Background(), cfg)

	require.Error(t, err, "all-empty providers must fail loudly")
	assert.Nil(t, responses)
	assert.Len(t, failed, 2)
	assert.Contains(t, err.Error(), "모든 프로바이더가 실패했습니다")
	assert.Contains(t, err.Error(), "empty output")
}

// TestRunDebate_JudgeRunsWithPartialResponses verifies that the judge runs
// even when only a subset of debaters successfully respond.
// Reproduces: judge never ran because claude+codex were empty → all 3 deemed missing.
func TestRunDebate_JudgeRunsWithPartialResponses(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	// Only one debater succeeds; judge is a real binary (cat).
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("gemini"),
			emptyOutputProvider("claude"),
		},
		Strategy:       StrategyDebate,
		Prompt:         "topic",
		TimeoutSeconds: 10,
		// Judge is 'cat' which is always installed.
		JudgeProvider: "cat-judge",
	}

	// Override judge lookup: inject a cat-based judge into providers for test isolation.
	// findOrBuildJudgeConfig falls back to Binary=JudgeProvider when not in Providers,
	// so we add it explicitly.
	cfg.Providers = append(cfg.Providers, ProviderConfig{
		Name:          "cat-judge",
		Binary:        "cat",
		Args:          []string{},
		PromptViaArgs: false,
	})

	responses, _, _, err := runDebate(context.Background(), cfg)
	require.NoError(t, err)

	// Judge response should be present even with partial debater success.
	judgeFound := false
	for _, r := range responses {
		if r.Provider == "cat-judge (judge)" {
			judgeFound = true
			break
		}
	}
	assert.True(t, judgeFound, "judge must run and append its response even with partial debaters")
}

// Tests for debate/rebuttal failed-provider surfacing live in
// debate_failed_providers_test.go to keep this file under the 300-line limit.
