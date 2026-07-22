package terminal

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCmuxAdapter_SurfaceTargetedCommands_WithEnvironmentWorkspace_AddWorkspaceArgument(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:13")

	tests := []struct {
		name string
		run  func(*CmuxAdapter) error
		want []string
	}{
		{
			name: "send command",
			run: func(adapter *CmuxAdapter) error {
				return adapter.SendCommand(context.Background(), "surface:1414", "echo ready")
			},
			want: []string{"send", "--workspace", "workspace:13", "--surface", "surface:1414", "--", "echo ready"},
		},
		{
			name: "send enter",
			run: func(adapter *CmuxAdapter) error {
				return adapter.SendCommand(context.Background(), "surface:1414", "\n")
			},
			want: []string{"send-key", "--workspace", "workspace:13", "--surface", "surface:1414", "Enter"},
		},
		{
			name: "read screen",
			run: func(adapter *CmuxAdapter) error {
				_, err := adapter.ReadScreen(context.Background(), "surface:1414", ReadScreenOpts{})
				return err
			},
			want: []string{"read-screen", "--workspace", "workspace:13", "--surface", "surface:1414"},
		},
		{
			name: "pipe pane start",
			run: func(adapter *CmuxAdapter) error {
				return adapter.PipePaneStart(context.Background(), "surface:1414", "/tmp/output.txt")
			},
			want: []string{"pipe-pane", "--workspace", "workspace:13", "--surface", "surface:1414", "--command", "cat >> '/tmp/output.txt'"},
		},
		{
			name: "pipe pane stop",
			run: func(adapter *CmuxAdapter) error {
				return adapter.PipePaneStop(context.Background(), "surface:1414")
			},
			want: []string{"pipe-pane", "--workspace", "workspace:13", "--surface", "surface:1414", "--command", ""},
		},
		{
			name: "focus pane",
			run: func(adapter *CmuxAdapter) error {
				return adapter.FocusPane(context.Background(), "surface:1414")
			},
			want: []string{"move-surface", "--workspace", "workspace:13", "--surface", "surface:1414", "--focus", "true"},
		},
		{
			name: "close surface",
			run: func(adapter *CmuxAdapter) error {
				return adapter.Close(context.Background(), "surface:1414")
			},
			want: []string{"close-surface", "--workspace", "workspace:13", "--surface", "surface:1414"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restore, captured := newCmuxMockV2("screen output", nil)
			defer restore()

			require.NoError(t, test.run(&CmuxAdapter{}))
			assert.Equal(t, test.want, captured.lastArgs())
		})
	}
}

func TestCmuxAdapter_SurfaceTargetedCommand_WithStoredWorkspace_OverridesEnvironment(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:13")
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	adapter := &CmuxAdapter{workspaceRef: "workspace:21"}
	err := adapter.SendCommand(context.Background(), "surface:1414", "echo ready")

	require.NoError(t, err)
	assert.Equal(t, []string{
		"send", "--workspace", "workspace:21", "--surface", "surface:1414", "--", "echo ready",
	}, captured.lastArgs())
}

func TestCmuxAdapter_WorkspaceScopedCommands_WithCanonicalUUID_AddWorkspaceArgument(t *testing.T) {
	const workspaceUUID = "CEC228BD-3FD5-4D00-8790-808F281E7CC1"
	t.Setenv("CMUX_WORKSPACE_ID", workspaceUUID)

	tests := []struct {
		name   string
		output string
		run    func(*CmuxAdapter) error
		want   []string
	}{
		{
			name:   "new surface",
			output: "OK surface:1414 workspace:13",
			run: func(adapter *CmuxAdapter) error {
				_, err := adapter.CreateSurface(context.Background())
				return err
			},
			want: []string{"new-surface", "--workspace", workspaceUUID},
		},
		{
			name:   "new split",
			output: "OK surface:1414 workspace:13",
			run: func(adapter *CmuxAdapter) error {
				_, err := adapter.SplitPane(context.Background(), Horizontal)
				return err
			},
			want: []string{"new-split", "--workspace", workspaceUUID, "right"},
		},
		{
			name: "notify",
			run: func(adapter *CmuxAdapter) error {
				return adapter.Notify(context.Background(), "ready")
			},
			want: []string{"notify", "--workspace", workspaceUUID, "--title", "ready"},
		},
		{
			name:   "surface health",
			output: "surface:1414 type=terminal in_window=true",
			run: func(adapter *CmuxAdapter) error {
				_, err := adapter.SurfaceHealth(context.Background(), "surface:1414")
				return err
			},
			want: []string{"surface-health", "--workspace", workspaceUUID},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restore, captured := newCmuxMockV2(test.output, nil)
			defer restore()

			require.NoError(t, test.run(&CmuxAdapter{}))
			assert.Equal(t, test.want, captured.lastArgs())
		})
	}
}

func TestCmuxAdapter_SendLongText_WithWorkspace_AddsContextOnlyToSurfaceCommand(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:13")
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	text := "prompt"
	err := (&CmuxAdapter{}).SendLongText(context.Background(), "surface:1414", text)

	require.NoError(t, err)
	require.Len(t, captured.calls, 3)
	bufferName := captured.calls[0].args[2]
	assert.Equal(t, []string{"set-buffer", "--name", bufferName, "--", text}, captured.calls[0].args)
	assert.Equal(t, []string{
		"paste-buffer", "--workspace", "workspace:13", "--surface", "surface:1414",
		"--name", bufferName,
	}, captured.calls[1].args)
	assert.Equal(t, []string{
		"set-buffer", "--name", bufferName, "--", cmuxInputBufferSentinel,
	}, captured.calls[2].args)
}

func TestCmuxAdapter_SendLongTextFallback_WithWorkspace_AddsContextToSend(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:13")

	tests := []struct {
		name        string
		failCommand string
	}{
		{name: "set buffer failure", failCommand: "set-buffer"},
		{name: "paste buffer failure", failCommand: "paste-buffer"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			captured := newCmuxSelectiveFailureMock(t, test.failCommand)

			err := (&CmuxAdapter{}).SendLongText(context.Background(), "surface:1414", "fallback")

			require.NoError(t, err)
			assert.Contains(t, captured.calls, struct {
				name string
				args []string
			}{
				name: "cmux",
				args: []string{"send", "--workspace", "workspace:13", "--surface", "surface:1414", "--", "fallback"},
			})
			for _, call := range captured.calls {
				if call.args[0] == "set-buffer" {
					assert.NotContains(t, call.args, "--workspace")
				}
			}
		})
	}
}

func newCmuxSelectiveFailureMock(t *testing.T, failCommand string) *capturedCmds {
	t.Helper()
	original := execCommand
	originalContext := execCommandContext
	captured := &capturedCmds{}
	buildCommand := func(name string, args ...string) *exec.Cmd {
		captured.calls = append(captured.calls, struct {
			name string
			args []string
		}{name: name, args: args})
		if len(args) > 0 && args[0] == failCommand {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	execCommand = buildCommand
	execCommandContext = func(_ context.Context, name string, args ...string) *exec.Cmd {
		return buildCommand(name, args...)
	}
	t.Cleanup(func() {
		execCommand = original
		execCommandContext = originalContext
	})
	return captured
}
