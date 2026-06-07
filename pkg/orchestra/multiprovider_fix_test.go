package orchestra

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// orchestrationTimeout must SUM per-provider budgets for sequential strategies
// (pipeline, relay) so later providers do not start with an already-expired
// context. This is the same failure class as the spec-review 0/N watchdog bug.

func TestOrchestrationTimeout_PipelineSumsPerProviderTimeouts(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyPipeline,
		Providers: []ProviderConfig{
			{Name: "a", ExecutionTimeout: 100 * time.Second},
			{Name: "b", ExecutionTimeout: 100 * time.Second},
			{Name: "c", ExecutionTimeout: 100 * time.Second},
		},
		TimeoutSeconds: 120,
	}
	// Sequential pipeline: global deadline must be the SUM (300s), not MAX (100s).
	assert.Equal(t, 300*time.Second, orchestrationTimeout(cfg))
}

func TestOrchestrationTimeout_RelaySumsPerProviderTimeouts(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyRelay,
		Providers: []ProviderConfig{
			{Name: "a", ExecutionTimeout: 200 * time.Second},
			{Name: "b", ExecutionTimeout: 100 * time.Second},
		},
		TimeoutSeconds: 60,
	}
	// Relay is sequential: SUM (300s), not MAX (200s).
	assert.Equal(t, 300*time.Second, orchestrationTimeout(cfg))
}

func TestOrchestrationTimeout_ConsensusUsesMaxNotSum(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyConsensus,
		Providers: []ProviderConfig{
			{Name: "a", ExecutionTimeout: 200 * time.Second},
			{Name: "b", ExecutionTimeout: 100 * time.Second},
		},
		TimeoutSeconds: 60,
	}
	// Parallel consensus overlaps, so the max provider budget bounds the run.
	assert.Equal(t, 200*time.Second, orchestrationTimeout(cfg))
}

func TestOrchestrationTimeout_DebateMaxTimesPhaseCountUnchanged(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyDebate,
		Providers: []ProviderConfig{
			{Name: "a", ExecutionTimeout: 100 * time.Second},
			{Name: "b", ExecutionTimeout: 100 * time.Second},
		},
		TimeoutSeconds: 60,
		DebateRounds:   2,
		JudgeProvider:  "claude",
	}
	// MAX(100) * phaseCount(base 1 + rounds>=2 + judge = 3) = 300s.
	assert.Equal(t, 300*time.Second, orchestrationTimeout(cfg))
}

// runFastest must not treat an exit-0 provider with empty stdout as the winning
// response (false green). It mirrors runParallel's EmptyOutput rejection.
func TestRunOrchestra_FastestEmptyOutputIsNotSuccess(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX sh fixture")
	}
	dir := t.TempDir()
	empty := writeOutputProvider(t, dir, "empty", "")
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{{Name: "empty", Binary: empty}},
		Strategy:       StrategyFastest,
		Prompt:         "x",
		TimeoutSeconds: 5,
	}
	_, err := RunOrchestra(context.Background(), cfg)
	assert.Error(t, err, "empty-output provider must not be a successful fastest response")
}

// setupStdin must release the parent's /dev/null descriptor after Start so that
// repeated PromptViaArgs subprocess executions do not leak file descriptors.
func TestSetupStdin_PromptViaArgsClosesDevNull(t *testing.T) {
	t.Parallel()
	fake := &fakeCommand{waitCh: make(chan error, 1)}
	err := setupStdin(fake, ProviderRequest{
		Provider: "gemini",
		Config:   ProviderConfig{Name: "gemini", Binary: "gemini", PromptViaArgs: true},
	})
	require.NoError(t, err)
	f, ok := fake.stdin.(*os.File)
	require.True(t, ok, "PromptViaArgs stdin must be the devnull *os.File")
	_, readErr := f.Read(make([]byte, 1))
	assert.ErrorIs(t, readErr, os.ErrClosed, "parent devnull fd must be closed after Start")
}
