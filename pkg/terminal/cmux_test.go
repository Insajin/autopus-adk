package terminal

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturedCmd records the last exec command for assertion.
// Shared with tmux_test.go via same package scope.
type capturedCmd struct {
	name string
	args []string
	err  error
}

// capturedCmds records all exec commands for multi-call assertion.
type capturedCmds struct {
	calls []struct {
		name string
		args []string
	}
}

func (c *capturedCmds) lastArgs() []string {
	if len(c.calls) == 0 {
		return nil
	}
	return c.calls[len(c.calls)-1].args
}

func (c *capturedCmds) lastName() string {
	if len(c.calls) == 0 {
		return ""
	}
	return c.calls[len(c.calls)-1].name
}

// newCmuxMockV2 replaces execCommand with a mock that records all calls and returns
// configurable output. output is returned by cmd.Output() calls via printf.
func newCmuxMockV2(output string, returnErr error) (restore func(), captured *capturedCmds) {
	orig := execCommand
	cap := &capturedCmds{}
	execCommand = func(name string, args ...string) *exec.Cmd {
		cap.calls = append(cap.calls, struct {
			name string
			args []string
		}{name, args})
		if returnErr != nil {
			return exec.Command("false")
		}
		if output != "" {
			return exec.Command("printf", "%s", output)
		}
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

// TestCmuxAdapter_CreateWorkspace verifies new-workspace and rename-workspace are called correctly.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_CreateWorkspace(t *testing.T) {
	restore, captured := newCmuxMockV2("OK workspace:5", nil)
	defer restore()

	a := &CmuxAdapter{}
	err := a.CreateWorkspace(context.Background(), "my-workspace")
	require.NoError(t, err)
	require.Len(t, captured.calls, 2)
	// First call: cmux new-workspace
	assert.Equal(t, "cmux", captured.calls[0].name)
	assert.Contains(t, captured.calls[0].args, "new-workspace")
	// Second call: cmux rename-workspace --workspace workspace:5 my-workspace
	assert.Equal(t, "cmux", captured.calls[1].name)
	assert.Contains(t, captured.calls[1].args, "rename-workspace")
	assert.Contains(t, captured.calls[1].args, "workspace:5")
	assert.Contains(t, captured.calls[1].args, "my-workspace")
}

// TestCmuxAdapter_SplitPane_Horizontal verifies new-split right is called and surface ref is returned.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_SplitPane_Horizontal(t *testing.T) {
	restore, captured := newCmuxMockV2("OK surface:7 workspace:1", nil)
	defer restore()

	a := &CmuxAdapter{}
	paneID, err := a.SplitPane(context.Background(), Horizontal)
	require.NoError(t, err)
	assert.Equal(t, PaneID("surface:7"), paneID)
	assert.Equal(t, "cmux", captured.lastName())
	combined := strings.Join(captured.lastArgs(), " ")
	assert.Contains(t, combined, "new-split")
	assert.Contains(t, combined, "right")
}

// TestCmuxAdapter_SplitPane_Vertical verifies new-split down is called and surface ref is returned.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_SplitPane_Vertical(t *testing.T) {
	restore, captured := newCmuxMockV2("OK surface:8 workspace:1", nil)
	defer restore()

	a := &CmuxAdapter{}
	paneID, err := a.SplitPane(context.Background(), Vertical)
	require.NoError(t, err)
	assert.Equal(t, PaneID("surface:8"), paneID)
	combined := strings.Join(captured.lastArgs(), " ")
	assert.Contains(t, combined, "new-split")
	assert.Contains(t, combined, "down")
}

// TestCmuxAdapter_SendCommand verifies send --surface <ref> <cmd> is issued.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_SendCommand(t *testing.T) {
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	a := &CmuxAdapter{}
	err := a.SendCommand(context.Background(), "surface:7", "echo hello")
	require.NoError(t, err)
	combined := strings.Join(captured.lastArgs(), " ")
	assert.Contains(t, combined, "send")
	assert.Contains(t, combined, "--surface")
	assert.Contains(t, combined, "surface:7")
	assert.Contains(t, combined, "echo hello")
}

// TestCmuxAdapter_Notify verifies notify --title <msg> is issued.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_Notify(t *testing.T) {
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	a := &CmuxAdapter{}
	err := a.Notify(context.Background(), "build complete")
	require.NoError(t, err)
	combined := strings.Join(captured.lastArgs(), " ")
	assert.Contains(t, combined, "notify")
	assert.Contains(t, combined, "--title")
	assert.Contains(t, combined, "build complete")
}

// TestCmuxAdapter_Close_SurfaceRef verifies close-surface --surface <ref> for surface refs.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_Close_SurfaceRef(t *testing.T) {
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	a := &CmuxAdapter{}
	err := a.Close(context.Background(), "surface:7")
	require.NoError(t, err)
	combined := strings.Join(captured.lastArgs(), " ")
	assert.Contains(t, combined, "close-surface")
	assert.Contains(t, combined, "--surface")
	assert.Contains(t, combined, "surface:7")
}

// TestCmuxAdapter_Close_WorkspaceName verifies close-workspace uses stored ref after CreateWorkspace.
// Note: cannot use t.Parallel() — this test mutates the package-level execCommand variable.
func TestCmuxAdapter_Close_WorkspaceName(t *testing.T) {
	restore, captured := newCmuxMockV2("OK workspace:5", nil)
	defer restore()

	a := &CmuxAdapter{}
	// Populate workspaceRef via CreateWorkspace.
	_ = a.CreateWorkspace(context.Background(), "my-workspace")
	err := a.Close(context.Background(), "my-workspace")
	require.NoError(t, err)
	last := captured.calls[len(captured.calls)-1]
	combined := strings.Join(last.args, " ")
	assert.Contains(t, combined, "close-workspace")
	assert.Contains(t, combined, "--workspace")
	assert.Contains(t, combined, "workspace:5")
}
