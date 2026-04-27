package orchestra

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunParallelEmitsProgressForNonTTY(t *testing.T) {
	origNewCommand := newCommand
	origStderr := os.Stderr
	defer func() {
		newCommand = origNewCommand
		os.Stderr = origStderr
	}()

	readEnd, writeEnd, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = writeEnd

	newCommand = func(_ context.Context, name string, _ ...string) command {
		waitCh := make(chan error, 1)
		return &fakeCommand{
			waitCh:   waitCh,
			exitCode: 0,
			startFn: func(cmd *fakeCommand) error {
				_, _ = io.WriteString(cmd.stdout, name+" response")
				waitCh <- nil
				return nil
			},
		}
	}

	responses, failed, err := runParallel(context.Background(), OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "claude", PromptViaArgs: true},
			{Name: "codex", Binary: "codex", PromptViaArgs: true},
		},
		Prompt:         "progress smoke",
		TimeoutSeconds: 5,
	})
	require.NoError(t, err)
	require.Len(t, responses, 2)
	require.Empty(t, failed)

	require.NoError(t, writeEnd.Close())
	output, err := io.ReadAll(readEnd)
	require.NoError(t, err)
	text := string(output)
	assert.Contains(t, text, "claude")
	assert.Contains(t, text, "codex")
	assert.Contains(t, text, "running")
}
