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

// capturedCmd records the last exec command for assertion.
type capturedCmd struct {
	name string
	args []string
	err  error
}

// mockExecCommand returns a helper that captures exec calls and injects behavior.
func newCmuxMock(returnErr error) (restore func(), captured *capturedCmd) {
	orig := execCommand
	cap := &capturedCmd{}
	execCommand = func(name string, args ...string) *exec.Cmd {
		cap.name = name
		cap.args = args
		cap.err = returnErr
		if returnErr != nil {
			// Return a command that will fail.
			return exec.Command("false")
		}
		// Return a no-op command.
		return exec.Command("true")
	}
	return func() { execCommand = orig }, cap
}

// TestCmuxAdapter_Name verifies Name returns "cmux".
func TestCmuxAdapter_Name(t *testing.T) {
	t.Parallel()

	a := &CmuxAdapter{}
	assert.Equal(t, "cmux", a.Name())
}

// TestCmuxAdapter_CreateWorkspace verifies correct cmux workspace create command is run.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_CreateWorkspace(t *testing.T) {
	restore, captured := newCmuxMock(nil)
	defer restore()

	a := &CmuxAdapter{}
	err := a.CreateWorkspace(context.Background(), "my-workspace")
	require.NoError(t, err)
	assert.Equal(t, "cmux", captured.name)
	assert.Contains(t, captured.args, "workspace")
	assert.Contains(t, captured.args, "create")
	assert.Contains(t, captured.args, "my-workspace")
}

// TestCmuxAdapter_SplitPane_Horizontal verifies the horizontal direction flag is passed.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_SplitPane_Horizontal(t *testing.T) {
	restore, captured := newCmuxMock(nil)
	defer restore()

	a := &CmuxAdapter{}
	_, err := a.SplitPane(context.Background(), Horizontal)
	require.NoError(t, err)
	assert.Equal(t, "cmux", captured.name)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "h", "horizontal direction must pass 'h' flag")
}

// TestCmuxAdapter_SplitPane_Vertical verifies the vertical direction flag is passed.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_SplitPane_Vertical(t *testing.T) {
	restore, captured := newCmuxMock(nil)
	defer restore()

	a := &CmuxAdapter{}
	_, err := a.SplitPane(context.Background(), Vertical)
	require.NoError(t, err)
	assert.Equal(t, "cmux", captured.name)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "v", "vertical direction must pass 'v' flag")
}

// TestCmuxAdapter_SendCommand verifies correct pane ID and command are passed.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_SendCommand(t *testing.T) {
	restore, captured := newCmuxMock(nil)
	defer restore()

	a := &CmuxAdapter{}
	err := a.SendCommand(context.Background(), "pane-1", "echo hello")
	require.NoError(t, err)
	assert.Equal(t, "cmux", captured.name)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "pane-1")
	assert.Contains(t, combined, "echo hello")
}

// TestCmuxAdapter_Notify verifies the notify command is issued with the message.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_Notify(t *testing.T) {
	restore, captured := newCmuxMock(nil)
	defer restore()

	a := &CmuxAdapter{}
	err := a.Notify(context.Background(), "build complete")
	require.NoError(t, err)
	assert.Equal(t, "cmux", captured.name)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "notify")
	assert.Contains(t, combined, "build complete")
}

// TestCmuxAdapter_Close verifies the workspace remove command is run.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_Close(t *testing.T) {
	restore, captured := newCmuxMock(nil)
	defer restore()

	a := &CmuxAdapter{}
	err := a.Close(context.Background(), "my-workspace")
	require.NoError(t, err)
	assert.Equal(t, "cmux", captured.name)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "remove")
	assert.Contains(t, combined, "my-workspace")
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
	restore, _ := newCmuxMock(fmt.Errorf("cmux: send failed"))
	defer restore()

	a := &CmuxAdapter{}
	err := a.SendCommand(context.Background(), "pane-1", "bad-cmd")
	assert.Error(t, err, "SendCommand must return an error when command fails")
	assert.Contains(t, err.Error(), "send command")
}

// TestCmuxAdapter_Notify_Error verifies that Notify propagates command execution errors.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_Notify_Error(t *testing.T) {
	restore, _ := newCmuxMock(fmt.Errorf("cmux: notify failed"))
	defer restore()

	a := &CmuxAdapter{}
	err := a.Notify(context.Background(), "msg")
	assert.Error(t, err, "Notify must return an error when command fails")
	assert.Contains(t, err.Error(), "notify")
}

// TestCmuxAdapter_Close_Error verifies that Close propagates command execution errors.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_Close_Error(t *testing.T) {
	restore, _ := newCmuxMock(fmt.Errorf("cmux: remove failed"))
	defer restore()

	a := &CmuxAdapter{}
	err := a.Close(context.Background(), "my-workspace")
	assert.Error(t, err, "Close must return an error when command fails")
	assert.Contains(t, err.Error(), "remove workspace")
}
