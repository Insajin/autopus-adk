package terminal

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCmuxMock replaces both command constructors with a mock that captures calls.
// Used by error-path tests that only need a single call and don't require output.
func newCmuxMock(returnErr error) (restore func(), captured *capturedCmd) {
	orig := execCommand
	origCtx := execCommandContext
	cap := &capturedCmd{}
	buildCmd := func(name string, args ...string) *exec.Cmd {
		cap.name = name
		cap.args = args
		cap.err = returnErr
		if returnErr != nil {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	execCommand = buildCmd
	execCommandContext = func(_ context.Context, name string, args ...string) *exec.Cmd {
		return buildCmd(name, args...)
	}
	return func() {
		execCommand = orig
		execCommandContext = origCtx
	}, cap
}

// TestCmuxAdapter_CreateWorkspace_Error verifies command failures are propagated.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_CreateWorkspace_Error(t *testing.T) {
	restore, _ := newCmuxMock(fmt.Errorf("cmux: workspace already exists"))
	defer restore()

	a := &CmuxAdapter{}
	err := a.CreateWorkspace(context.Background(), "duplicate")
	assert.Error(t, err, "CreateWorkspace must return an error when command fails")
}

// TestCmuxAdapter_SplitPane_Error verifies that SplitPane propagates command execution errors.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_SplitPane_Error(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	restore, _ := newCmuxMock(fmt.Errorf("cmux: split failed"))
	defer restore()

	a := &CmuxAdapter{}
	_, err := a.SplitPane(context.Background(), Horizontal)
	assert.Error(t, err, "SplitPane must return an error when command fails")
	assert.Contains(t, err.Error(), "split pane")
}

// TestCmuxAdapter_SendCommand_Error verifies that SendCommand propagates command execution errors.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_SendCommand_Error(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	restore, _ := newCmuxMock(fmt.Errorf("cmux: send failed"))
	defer restore()

	a := &CmuxAdapter{}
	err := a.SendCommand(context.Background(), "surface:7", "bad-cmd")
	assert.Error(t, err, "SendCommand must return an error when command fails")
	assert.Contains(t, err.Error(), "send command")
}

// TestCmuxAdapter_Notify_Error verifies that Notify propagates command execution errors.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_Notify_Error(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	restore, _ := newCmuxMock(fmt.Errorf("cmux: notify failed"))
	defer restore()

	a := &CmuxAdapter{}
	err := a.Notify(context.Background(), "msg")
	assert.Error(t, err, "Notify must return an error when command fails")
	assert.Contains(t, err.Error(), "notify")
}

// TestCmuxAdapter_Close_Error verifies that Close propagates errors for surface refs.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_Close_Error(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	restore, _ := newCmuxMock(fmt.Errorf("cmux: remove failed"))
	defer restore()

	a := &CmuxAdapter{}
	err := a.Close(context.Background(), "surface:7")
	assert.Error(t, err, "Close must return an error when command fails")
	assert.Contains(t, err.Error(), "close surface")
}

// TestCmuxAdapter_Close_NoWorkspaceRef verifies Close errors when no workspace ref is stored.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_Close_NoWorkspaceRef(t *testing.T) {
	restore, _ := newCmuxMockV2("", nil)
	defer restore()

	a := &CmuxAdapter{} // workspaceRef is empty
	err := a.Close(context.Background(), "my-workspace")
	assert.Error(t, err, "Close must return error when no workspace ref is stored")
	assert.Contains(t, err.Error(), "no workspace ref stored")
}

// TestCmuxAdapter_Close_WorkspaceRef verifies close-workspace for workspace: refs directly.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_Close_WorkspaceRef(t *testing.T) {
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	a := &CmuxAdapter{}
	err := a.Close(context.Background(), "workspace:5")
	require.NoError(t, err)
	combined := strings.Join(captured.lastArgs(), " ")
	assert.Contains(t, combined, "close-workspace")
	assert.Contains(t, combined, "--workspace")
	assert.Contains(t, combined, "workspace:5")
}

// TestCmuxAdapter_CreateWorkspace_ParseFail verifies CreateWorkspace errors on unparseable output.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_CreateWorkspace_ParseFail(t *testing.T) {
	restore, _ := newCmuxMockV2("OK", nil) // output missing workspace ref
	defer restore()

	a := &CmuxAdapter{}
	err := a.CreateWorkspace(context.Background(), "my-workspace")
	assert.Error(t, err, "CreateWorkspace must return error when output cannot be parsed")
	assert.Contains(t, err.Error(), "failed to parse workspace ref")
}

// TestCmuxAdapter_SplitPane_ParseFail verifies SplitPane errors when output has no surface ref.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_SplitPane_ParseFail(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	restore, _ := newCmuxMockV2("OK", nil) // output missing surface ref
	defer restore()

	a := &CmuxAdapter{}
	_, err := a.SplitPane(context.Background(), Horizontal)
	assert.Error(t, err, "SplitPane must return error when output cannot be parsed")
	assert.Contains(t, err.Error(), "failed to parse surface ref")
}

// TestValidatePaneID_InvalidFormat verifies that validatePaneID rejects invalid formats.
func TestValidatePaneID_InvalidFormat(t *testing.T) {
	t.Parallel()

	err := validatePaneID("invalid pane!")
	assert.Error(t, err, "validatePaneID must reject invalid formats")
}
