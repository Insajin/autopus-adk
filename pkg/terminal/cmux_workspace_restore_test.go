package terminal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCmuxAdapterWithWorkspace_WithCanonicalReference_RestoresContext(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:99")

	tests := []struct {
		name         string
		workspaceRef string
	}{
		{name: "workspace reference", workspaceRef: "workspace:13"},
		{name: "canonical UUID", workspaceRef: "CEC228BD-3FD5-4D00-8790-808F281E7CC1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter, err := NewCmuxAdapterWithWorkspace(test.workspaceRef)
			require.NoError(t, err)

			workspaceRef, err := adapter.WorkspaceRef()
			require.NoError(t, err)
			assert.Equal(t, test.workspaceRef, workspaceRef)
		})
	}
}

func TestNewCmuxAdapterWithWorkspace_WithInvalidReference_ReturnsError(t *testing.T) {
	for _, workspaceRef := range []string{"", "surface:13", "workspace:abc", "not-a-uuid"} {
		t.Run(workspaceRef, func(t *testing.T) {
			adapter, err := NewCmuxAdapterWithWorkspace(workspaceRef)

			assert.Nil(t, adapter)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid workspace ref")
		})
	}
}

func TestCmuxAdapter_WithWorkspaceRef_ClonesWithoutMutatingOriginal(t *testing.T) {
	original, err := NewCmuxAdapterWithWorkspace("workspace:13")
	require.NoError(t, err)

	cloneTerminal, err := original.WithWorkspaceRef("workspace:21")
	require.NoError(t, err)
	clone, ok := cloneTerminal.(*CmuxAdapter)
	require.True(t, ok)

	originalRef, err := original.WorkspaceRef()
	require.NoError(t, err)
	cloneRef, err := clone.WorkspaceRef()
	require.NoError(t, err)
	assert.Equal(t, "workspace:13", originalRef)
	assert.Equal(t, "workspace:21", cloneRef)
}

func TestCmuxAdapter_WorkspaceRef_WithoutStoredReference_UsesEnvironment(t *testing.T) {
	const workspaceUUID = "CEC228BD-3FD5-4D00-8790-808F281E7CC1"
	t.Setenv("CMUX_WORKSPACE_ID", workspaceUUID)

	workspaceRef, err := (&CmuxAdapter{}).WorkspaceRef()

	require.NoError(t, err)
	assert.Equal(t, workspaceUUID, workspaceRef)
}

func TestCmuxAdapter_SurfaceCommand_WithInvalidWorkspace_FailsBeforeExecution(t *testing.T) {
	tests := []struct {
		name          string
		stored        string
		environment   string
		wantErrorPart string
	}{
		{
			name:          "missing environment",
			wantErrorPart: "workspace context is unavailable",
		},
		{
			name:          "invalid environment",
			environment:   "surface:13",
			wantErrorPart: "invalid CMUX_WORKSPACE_ID",
		},
		{
			name:          "invalid stored reference does not use valid environment",
			stored:        "workspace:invalid",
			environment:   "workspace:13",
			wantErrorPart: "invalid stored workspace context",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("CMUX_WORKSPACE_ID", test.environment)
			restore, captured := newCmuxMockV2("", nil)
			defer restore()

			err := (&CmuxAdapter{workspaceRef: test.stored}).SendCommand(
				context.Background(), "surface:1414", "echo ready",
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErrorPart)
			assert.Empty(t, captured.calls, "invalid workspace must fail before cmux execution")
		})
	}
}

func TestCmuxAdapter_BufferAndSignalCommands_WithWorkspace_DoNotAddUnsupportedFlag(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:13")
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	adapter := &CmuxAdapter{}
	require.NoError(t, adapter.WaitForSignal(context.Background(), "ready", time.Second))
	require.NoError(t, adapter.SendSignal(context.Background(), "ready"))

	require.Len(t, captured.calls, 2)
	assert.Equal(t, []string{"wait-for", "ready", "--timeout", "30"}, captured.calls[0].args)
	assert.Equal(t, []string{"wait-for", "-S", "ready"}, captured.calls[1].args)
	assert.NotContains(t, captured.calls[0].args, "--workspace")
	assert.NotContains(t, captured.calls[1].args, "--workspace")
}
