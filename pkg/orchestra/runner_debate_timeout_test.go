package orchestra

import (
	"context"
	"io"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunOrchestra_DebateJudgeRunsAfterProviderTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	origNewCommand := newCommand
	origWaitGrace := providerWaitGracePeriod
	defer func() {
		newCommand = origNewCommand
		providerWaitGracePeriod = origWaitGrace
	}()

	var echoCalls atomic.Int32
	signal := func(ch chan error, err error) {
		select {
		case ch <- err:
		default:
		}
	}

	newCommand = func(_ context.Context, name string, _ ...string) command {
		switch name {
		case "echo":
			call := echoCalls.Add(1)
			waitCh := make(chan error, 1)
			return &fakeCommand{
				waitCh:   waitCh,
				exitCode: 0,
				startFn: func(cmd *fakeCommand) error {
					if call == 1 {
						_, _ = io.WriteString(cmd.stdout, "fast response")
						signal(waitCh, nil)
						return nil
					}
					_, _ = io.WriteString(cmd.stdout, "judge response")
					time.AfterFunc(50*time.Millisecond, func() { signal(waitCh, nil) })
					return nil
				},
				terminateFn: func(_ *fakeCommand, _ string) error {
					signal(waitCh, context.Canceled)
					return nil
				},
			}
		case "sleep":
			waitCh := make(chan error, 1)
			return &fakeCommand{
				waitCh:   waitCh,
				exitCode: -1,
				terminateFn: func(_ *fakeCommand, _ string) error {
					signal(waitCh, context.DeadlineExceeded)
					return nil
				},
			}
		default:
			t.Fatalf("unexpected provider binary: %s", name)
			return nil
		}
	}
	providerWaitGracePeriod = 20 * time.Millisecond

	result, err := RunOrchestra(context.Background(), OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "fast", Binary: "echo", PromptViaArgs: true},
			{Name: "slow", Binary: "sleep", PromptViaArgs: true},
		},
		Prompt:         "debate timeout then judge",
		Strategy:       StrategyDebate,
		TimeoutSeconds: 1,
		DebateRounds:   1,
		JudgeProvider:  "fast",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.FailedProviders, 1)
	assert.Equal(t, "slow", result.FailedProviders[0].Name)
	require.Len(t, result.Responses, 2)
	assert.Equal(t, "fast", result.Responses[0].Provider)
	assert.Equal(t, "fast (judge)", result.Responses[1].Provider)
	assert.Contains(t, result.Responses[1].Output, "judge response")
}
