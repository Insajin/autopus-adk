package orchestra

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunProvider_CodexExecReadsOutputLastMessage(t *testing.T) {
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
			if _, err := io.WriteString(cmd.stdout, "codex telemetry\nnot the final answer\n"); err != nil {
				return err
			}
			return os.WriteFile(lastMessagePath, []byte("final brainstorm answer"), 0600)
		},
	}

	newCommand = func(_ context.Context, _ string, args ...string) command {
		capturedArgs = append([]string{}, args...)
		return fake
	}

	resp, err := runProvider(context.Background(), ProviderConfig{
		Name:   "codex",
		Binary: "codex",
		Args:   []string{"exec", "--sandbox", "workspace-write", "-m", "gpt-5.5"},
	}, "brainstorm prompt")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "final brainstorm answer", resp.Output)
	assert.Contains(t, capturedArgs, "--output-last-message")
	assert.Equal(t, "brainstorm prompt", fake.stdinBuf.String())
}
