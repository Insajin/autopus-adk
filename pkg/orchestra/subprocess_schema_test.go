package orchestra

import (
	"context"
	"io"
	"testing"

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
			Args: []string{"exec", "--full-auto"},
		},
	})

	assert.Equal(t, []string{"exec", "--full-auto"}, args)
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
