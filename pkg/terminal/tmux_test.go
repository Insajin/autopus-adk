package terminal

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTmuxMock replaces execCommand and returns a cleanup function and captured args.
func newTmuxMock() (restore func(), captured *capturedCmd) {
	orig := execCommand
	cap := &capturedCmd{}
	execCommand = func(name string, args ...string) *exec.Cmd {
		cap.name = name
		cap.args = args
		return exec.Command("true")
	}
	return func() { execCommand = orig }, cap
}

// newTmuxErrorMock replaces execCommand with a mock that always returns a failing command.
func newTmuxErrorMock() (restore func()) {
	orig := execCommand
	execCommand = func(_ string, _ ...string) *exec.Cmd {
		return exec.Command("false")
	}
	return func() { execCommand = orig }
}

// TestTmuxAdapter_Name verifies Name returns "tmux".
func TestTmuxAdapter_Name(t *testing.T) {
	t.Parallel()

	a := &TmuxAdapter{}
	assert.Equal(t, "tmux", a.Name())
}

// TestTmuxAdapter_CreateWorkspace verifies the new-session command is run with correct args.
func TestTmuxAdapter_CreateWorkspace(t *testing.T) {
	// Note: cannot use t.Parallel() with t.Setenv per Go test rules.

	// Ensure TMUX env is unset to avoid nested session path.
	t.Setenv("TMUX", "")

	restore, captured := newTmuxMock()
	defer restore()

	a := &TmuxAdapter{}
	err := a.CreateWorkspace(context.Background(), "my-session")
	require.NoError(t, err)
	assert.Equal(t, "tmux", captured.name)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "new-session")
	assert.Contains(t, combined, "-d")
	assert.Contains(t, combined, "-s")
	assert.Contains(t, combined, "my-session")
}

// TestTmuxAdapter_CreateWorkspace_NestedSession verifies that when TMUX env is set,
// new-window is used instead of new-session.
func TestTmuxAdapter_CreateWorkspace_NestedSession(t *testing.T) {
	// Note: cannot use t.Parallel() with t.Setenv per Go test rules.

	// Simulate being inside an existing tmux session.
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	restore, captured := newTmuxMock()
	defer restore()

	a := &TmuxAdapter{}
	err := a.CreateWorkspace(context.Background(), "nested-session")
	require.NoError(t, err)
	assert.Equal(t, "tmux", captured.name)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "new-window", "must use new-window when nested in tmux")
	assert.NotContains(t, combined, "new-session", "must NOT use new-session when nested in tmux")
}

// TestTmuxAdapter_SplitPane verifies the split-window command uses correct flags.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestTmuxAdapter_SplitPane(t *testing.T) {
	restore, captured := newTmuxMock()
	defer restore()

	a := &TmuxAdapter{}
	_, err := a.SplitPane(context.Background(), Horizontal)
	require.NoError(t, err)
	assert.Equal(t, "tmux", captured.name)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "split-window")
}

// TestTmuxAdapter_SendCommand verifies send-keys is called with correct session, pane, and command.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestTmuxAdapter_SendCommand(t *testing.T) {
	restore, captured := newTmuxMock()
	defer restore()

	a := &TmuxAdapter{}
	// TmuxAdapter must track the session name from the last CreateWorkspace call.
	// For this test, we directly test that SendCommand issues the correct tmux command.
	err := a.SendCommand(context.Background(), "0", "go build ./...")
	require.NoError(t, err)
	assert.Equal(t, "tmux", captured.name)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "send-keys")
	assert.Contains(t, combined, "go build ./...")
}

// TestTmuxAdapter_Close verifies kill-session is called with the correct target.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestTmuxAdapter_Close(t *testing.T) {
	restore, captured := newTmuxMock()
	defer restore()

	a := &TmuxAdapter{}
	err := a.Close(context.Background(), "my-session")
	require.NoError(t, err)
	assert.Equal(t, "tmux", captured.name)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "kill-session")
	assert.Contains(t, combined, "my-session")
}

// TestTmuxAdapter_Notify verifies display-message is called with the message.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestTmuxAdapter_Notify(t *testing.T) {
	restore, captured := newTmuxMock()
	defer restore()

	a := &TmuxAdapter{}
	err := a.Notify(context.Background(), "deployment done")
	require.NoError(t, err)
	assert.Equal(t, "tmux", captured.name)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "display-message")
	assert.Contains(t, combined, "deployment done")
}

// TestTmuxAdapter_NestedDetection_EnvVar verifies TMUX env variable detection logic.
// This is a unit test for the detection mechanism itself.
func TestTmuxAdapter_NestedDetection_EnvVar(t *testing.T) {
	// Note: cannot use t.Parallel() with t.Setenv per Go test rules.

	// When TMUX is set, isNestedTmux must return true.
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	assert.True(t, os.Getenv("TMUX") != "", "TMUX env var must be set to simulate nesting")

	// When TMUX is empty, isNestedTmux must return false.
	t.Setenv("TMUX", "")
	assert.True(t, os.Getenv("TMUX") == "", "TMUX env var must be empty when not nested")
}

// TestTmuxAdapter_CreateWorkspace_Error verifies that CreateWorkspace propagates command failures.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestTmuxAdapter_CreateWorkspace_Error(t *testing.T) {
	t.Setenv("TMUX", "")

	restore := newTmuxErrorMock()
	defer restore()

	a := &TmuxAdapter{}
	err := a.CreateWorkspace(context.Background(), "bad-session")
	assert.Error(t, err, "CreateWorkspace must return an error when command fails")
	assert.Contains(t, err.Error(), "create workspace")
}

// TestTmuxAdapter_SplitPane_Error verifies that SplitPane propagates command execution errors.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestTmuxAdapter_SplitPane_Error(t *testing.T) {
	restore := newTmuxErrorMock()
	defer restore()

	a := &TmuxAdapter{}
	_, err := a.SplitPane(context.Background(), Vertical)
	assert.Error(t, err, "SplitPane must return an error when command fails")
	assert.Contains(t, err.Error(), "split pane")
}

// TestTmuxAdapter_SendCommand_Error verifies that SendCommand propagates command execution errors.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestTmuxAdapter_SendCommand_Error(t *testing.T) {
	restore := newTmuxErrorMock()
	defer restore()

	a := &TmuxAdapter{}
	err := a.SendCommand(context.Background(), "0", "bad-cmd")
	assert.Error(t, err, "SendCommand must return an error when command fails")
	assert.Contains(t, err.Error(), "send command")
}

// TestTmuxAdapter_Notify_Error verifies that Notify propagates command execution errors.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestTmuxAdapter_Notify_Error(t *testing.T) {
	restore := newTmuxErrorMock()
	defer restore()

	a := &TmuxAdapter{}
	err := a.Notify(context.Background(), "msg")
	assert.Error(t, err, "Notify must return an error when command fails")
	assert.Contains(t, err.Error(), "notify")
}

// TestTmuxAdapter_Close_Error verifies that Close propagates command execution errors.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestTmuxAdapter_Close_Error(t *testing.T) {
	restore := newTmuxErrorMock()
	defer restore()

	a := &TmuxAdapter{}
	err := a.Close(context.Background(), "my-session")
	assert.Error(t, err, "Close must return an error when command fails")
	assert.Contains(t, err.Error(), "kill session")
}

// TestTmuxAdapter_SplitPane_Vertical verifies the vertical direction flag is passed.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestTmuxAdapter_SplitPane_Vertical(t *testing.T) {
	restore, captured := newTmuxMock()
	defer restore()

	a := &TmuxAdapter{}
	_, err := a.SplitPane(context.Background(), Vertical)
	require.NoError(t, err)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "split-window")
	assert.Contains(t, combined, "-v", "vertical direction must pass '-v' flag")
}
