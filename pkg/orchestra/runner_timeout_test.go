package orchestra

import (
	"bytes"
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error {
	return nil
}

type fakeCommand struct {
	stdinBuf      bytes.Buffer
	stdout        io.Writer
	stderr        io.Writer
	waitCh        chan error
	exitCode      int
	startFn       func(*fakeCommand) error
	terminateFn   func(*fakeCommand, string) error
	terminateCall atomic.Int32
}

func (f *fakeCommand) StdinPipe() (io.WriteCloser, error) {
	return nopWriteCloser{Writer: &f.stdinBuf}, nil
}

func (f *fakeCommand) SetStdin(_ io.Reader) {}

func (f *fakeCommand) SetStdout(w io.Writer) {
	f.stdout = w
}

func (f *fakeCommand) SetStderr(w io.Writer) {
	f.stderr = w
}

func (f *fakeCommand) Start() error {
	if f.startFn != nil {
		return f.startFn(f)
	}
	return nil
}

func (f *fakeCommand) Wait() error {
	return <-f.waitCh
}

func (f *fakeCommand) ExitCode() int {
	return f.exitCode
}

func (f *fakeCommand) Terminate(reason string) error {
	f.terminateCall.Add(1)
	if f.terminateFn != nil {
		return f.terminateFn(f, reason)
	}
	return nil
}

func TestRunProvider_ContextCancelTerminatesBlockedWait(t *testing.T) {
	origNewCommand := newCommand
	origWaitGrace := providerWaitGracePeriod
	defer func() {
		newCommand = origNewCommand
		providerWaitGracePeriod = origWaitGrace
	}()

	waitCh := make(chan error, 1)
	fake := &fakeCommand{
		waitCh:   waitCh,
		exitCode: -1,
		startFn: func(cmd *fakeCommand) error {
			_, _ = io.WriteString(cmd.stdout, "partial output")
			return nil
		},
		terminateFn: func(_ *fakeCommand, _ string) error {
			waitCh <- context.DeadlineExceeded
			return nil
		},
	}

	newCommand = func(context.Context, string, ...string) command {
		return fake
	}
	providerWaitGracePeriod = 20 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	resp, err := runProvider(ctx, ProviderConfig{
		Name:          "stuck",
		Binary:        "stuck",
		PromptViaArgs: true,
	}, "prompt")
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.TimedOut)
	assert.Equal(t, int32(1), fake.terminateCall.Load())
	assert.Less(t, elapsed, 200*time.Millisecond)
}

func TestRunDebate_TimeoutProviderDoesNotHangCollection(t *testing.T) {
	origNewCommand := newCommand
	origWaitGrace := providerWaitGracePeriod
	defer func() {
		newCommand = origNewCommand
		providerWaitGracePeriod = origWaitGrace
	}()

	newCommand = func(_ context.Context, name string, _ ...string) command {
		switch name {
		case "fast":
			waitCh := make(chan error, 1)
			waitCh <- nil
			return &fakeCommand{
				waitCh:   waitCh,
				exitCode: 0,
				startFn: func(cmd *fakeCommand) error {
					_, _ = io.WriteString(cmd.stdout, "fast response")
					return nil
				},
			}
		case "stuck":
			return &fakeCommand{
				waitCh:   nil,
				exitCode: -1,
			}
		default:
			t.Fatalf("unexpected provider binary: %s", name)
			return nil
		}
	}
	providerWaitGracePeriod = 20 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	responses, _, _, err := runDebate(ctx, OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "fast", Binary: "fast", PromptViaArgs: true},
			{Name: "stuck", Binary: "stuck", PromptViaArgs: true},
		},
		Prompt:         "debate timeout test",
		Strategy:       StrategyDebate,
		TimeoutSeconds: 1,
		DebateRounds:   2,
	})
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Len(t, responses, 1)
	assert.Equal(t, "fast", responses[0].Provider)
	assert.Less(t, elapsed, 250*time.Millisecond)
}
