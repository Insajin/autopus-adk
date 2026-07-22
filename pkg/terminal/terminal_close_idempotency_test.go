package terminal

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCmuxAdapter_CloseSurface_AlreadyAbsentIsIdempotent(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:13")

	for _, stderr := range []string{
		"Error: not_found: Surface not found\n",
		"Error: Surface ref not found: surface:1414\n",
	} {
		t.Run(strings.TrimSpace(stderr), func(t *testing.T) {
			captured := mockTerminalCloseFailure(t, stderr)

			err := (&CmuxAdapter{}).Close(context.Background(), "surface:1414")

			require.NoError(t, err)
			assert.Equal(t, []string{
				"close-surface", "--workspace", "workspace:13", "--surface", "surface:1414",
			}, captured.args)
		})
	}
}

func TestCmuxAdapter_CloseSurface_OtherFailuresRemainErrors(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:13")

	for _, stderr := range []string{
		"Error: permission_denied: Permission denied\n",
		"Error: Socket not found at /tmp/cmux.sock\n",
		"Error: invalid_params: Invalid surface handle\n",
		"Error: Surface ref not found: surface:9999\n",
	} {
		t.Run(strings.TrimSpace(stderr), func(t *testing.T) {
			mockTerminalCloseFailure(t, stderr)

			err := (&CmuxAdapter{}).Close(context.Background(), "surface:1414")

			require.Error(t, err)
			assert.Contains(t, err.Error(), strings.TrimSpace(stderr))
		})
	}
}

func TestTmuxAdapter_ClosePane_AlreadyAbsentIsIdempotent(t *testing.T) {
	for _, stderr := range []string{
		"can't find pane: %42\n",
	} {
		t.Run(strings.TrimSpace(stderr), func(t *testing.T) {
			captured := mockTerminalCloseFailure(t, stderr)

			err := (&TmuxAdapter{}).Close(context.Background(), "%42")

			require.NoError(t, err)
			assert.Equal(t, []string{"kill-pane", "-t", "%42"}, captured.args)
		})
	}
}

func TestTmuxAdapter_ClosePane_OtherFailuresRemainErrors(t *testing.T) {
	for _, stderr := range []string{
		"no server running on /private/tmp/tmux-501/default\n",
		"permission denied\n",
		"failed to connect to server: Connection refused\n",
		"invalid pane ID: %42\n",
		"can't find pane: %99\n",
	} {
		t.Run(strings.TrimSpace(stderr), func(t *testing.T) {
			mockTerminalCloseFailure(t, stderr)

			err := (&TmuxAdapter{}).Close(context.Background(), "%42")

			require.Error(t, err)
			assert.Contains(t, err.Error(), strings.TrimSpace(stderr))
		})
	}
}

func mockTerminalCloseFailure(t *testing.T, stderr string) *capturedCmd {
	t.Helper()
	original := execCommand
	captured := &capturedCmd{}
	execCommand = func(name string, args ...string) *exec.Cmd {
		captured.name = name
		captured.args = args
		return exec.Command("sh", "-c", "printf '%s' \"$1\" >&2; exit 1", "terminal-close-test", stderr)
	}
	t.Cleanup(func() { execCommand = original })
	return captured
}
