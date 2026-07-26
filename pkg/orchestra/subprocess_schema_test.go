package orchestra

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSubprocessArgs_UsesConfiguredSchemaFlag(t *testing.T) {
	t.Parallel()

	args := buildSubprocessArgs(ProviderRequest{
		Prompt:     "hello",
		SchemaPath: "/tmp/schema.json",
		Config: ProviderConfig{
			Args:          []string{"run", "--prompt", ""},
			PromptViaArgs: true,
			SchemaFlag:    "--response-schema",
		},
	})

	assert.Equal(t, []string{
		"run",
		"--prompt",
		"hello",
		"--response-schema",
		"/tmp/schema.json",
	}, args)
}

func TestBuildSubprocessArgs_SkipsSchemaFlagWhenNotConfigured(t *testing.T) {
	t.Parallel()

	args := buildSubprocessArgs(ProviderRequest{
		SchemaPath: "/tmp/schema.json",
		Config: ProviderConfig{
			Args: []string{"exec", "--sandbox", "workspace-write"},
		},
	})

	assert.Equal(t, []string{"exec", "--sandbox", "workspace-write"}, args)
}

func TestSubprocessBackend_Execute_SkipsJSONValidationForTextOutput(t *testing.T) {
	origNewCommand := newCommand
	defer func() {
		newCommand = origNewCommand
	}()

	waitCh := make(chan error, 1)
	waitCh <- nil
	fake := &fakeCommand{
		waitCh:   waitCh,
		exitCode: 0,
		startFn: func(cmd *fakeCommand) error {
			_, _ = io.WriteString(cmd.stdout, "plain text output")
			return nil
		},
	}

	newCommand = func(context.Context, string, ...string) command {
		return fake
	}

	backend := NewSubprocessBackendImpl()
	resp, err := backend.Execute(context.Background(), ProviderRequest{
		Provider: "claude",
		Role:     "judge",
		Config: ProviderConfig{
			Name:         "claude",
			Binary:       "claude",
			OutputFormat: "text",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Error)
	assert.Equal(t, "plain text output", resp.Output)
}

func TestSubprocessBackend_Execute_ContextCancelTerminatesBlockedWait(t *testing.T) {
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

	backend := NewSubprocessBackendImpl()
	done := make(chan struct {
		resp *ProviderResponse
		err  error
	}, 1)

	go func() {
		resp, err := backend.Execute(context.Background(), ProviderRequest{
			Provider: "codex",
			Prompt:   "prompt",
			Timeout:  30 * time.Millisecond,
			Config: ProviderConfig{
				Name:   "codex",
				Binary: "codex",
			},
		})
		done <- struct {
			resp *ProviderResponse
			err  error
		}{resp: resp, err: err}
	}()

	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.NotNil(t, result.resp)
		assert.True(t, result.resp.TimedOut)
		assert.Equal(t, int32(1), fake.terminateCall.Load())
	case <-time.After(250 * time.Millisecond):
		waitCh <- context.DeadlineExceeded
		t.Fatal("subprocess backend did not terminate a blocked provider wait after context timeout")
	}
}

func TestSubprocessBackend_Execute_CodexReadsOutputLastMessage(t *testing.T) {
	origNewCommand := newCommand
	defer func() {
		newCommand = origNewCommand
	}()

	var capturedArgs []string
	waitCh := make(chan error, 1)
	waitCh <- nil
	fake := &fakeCommand{
		waitCh:   waitCh,
		exitCode: 0,
		startFn: func(cmd *fakeCommand) error {
			lastMessagePath := argValueAfter(capturedArgs, "--output-last-message")
			if lastMessagePath == "" {
				return fmt.Errorf("missing --output-last-message arg")
			}
			if _, err := io.WriteString(cmd.stdout, "codex\n{\"verdict\":\"PASS\"}\ntokens used\n1\n"); err != nil {
				return err
			}
			return os.WriteFile(lastMessagePath, []byte(`{"verdict":"PASS","summary":"ok","findings":[]}`), 0600)
		},
	}

	newCommand = func(_ context.Context, _ string, args ...string) command {
		capturedArgs = append([]string{}, args...)
		return fake
	}

	backend := NewSubprocessBackendImpl()
	resp, err := backend.Execute(context.Background(), ProviderRequest{
		Provider:   "codex",
		Prompt:     "Review this SPEC",
		SchemaPath: "/tmp/reviewer-schema.json",
		Role:       "reviewer",
		Config: ProviderConfig{
			Name:       "codex",
			Binary:     "codex",
			Args:       []string{"exec", "--sandbox", "workspace-write"},
			SchemaFlag: "--output-schema",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, `{"verdict":"PASS","summary":"ok","findings":[]}`, resp.Output)
	assert.Empty(t, resp.Error)
	assert.Contains(t, capturedArgs, "--output-schema")
	assert.Contains(t, capturedArgs, "--output-last-message")
	assert.Equal(t, "Review this SPEC", fake.stdinBuf.String())
}

type recordingBackend struct {
	requests []ProviderRequest
}

func (r *recordingBackend) Execute(_ context.Context, req ProviderRequest) (*ProviderResponse, error) {
	r.requests = append(r.requests, req)
	return &ProviderResponse{
		Provider: req.Provider,
		Output:   defaultOutput(req.Role),
	}, nil
}

func (r *recordingBackend) Name() string {
	return "recording"
}

// freshExecutionPerRequest models a new built-in execution for every recorded request.
func (r *recordingBackend) freshExecutionPerRequest() bool { return true }

func TestRunSubprocessPipeline_EmbedsPromptSchemaWhenProviderLacksSchemaFlag(t *testing.T) {
	t.Parallel()

	backend := &recordingBackend{}
	cfg := SubprocessPipelineConfig{
		Backend:   backend,
		Providers: []ProviderConfig{{Name: "claude", Binary: "echo"}},
		Topic:     "test topic",
		PromptData: PromptData{
			ProjectName:    "test",
			ProjectSummary: "test project",
			TechStack:      "Go",
			MustReadFiles:  []string{"go.mod"},
			Topic:          "test topic",
			MaxTurns:       5,
		},
		Rounds: 0,
		Judge:  ProviderConfig{Name: "judge", Binary: "echo"},
	}

	_, err := RunSubprocessPipeline(context.Background(), cfg)
	require.NoError(t, err)
	require.NotEmpty(t, backend.requests)
	assert.Contains(t, backend.requests[0].Prompt, "Required JSON structure:")
	assert.Contains(t, backend.requests[0].Prompt, "\"$schema\"")
}

func argValueAfter(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
