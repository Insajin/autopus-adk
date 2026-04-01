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

// TestCmuxAdapter_SendLongText_LongText_BufferPath verifies long text (>=500B) uses
// set-buffer/paste-buffer/delete-buffer instead of send.
func TestCmuxAdapter_SendLongText_LongText_BufferPath(t *testing.T) {
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	a := &CmuxAdapter{}
	longText := strings.Repeat("A", 2000)
	err := a.SendLongText(context.Background(), "surface:7", longText)
	require.NoError(t, err)
	// Expect 3 calls: set-buffer, paste-buffer, delete-buffer
	require.Len(t, captured.calls, 3, "long text should use buffer path (3 calls)")
	// Call 1: set-buffer
	assert.Contains(t, strings.Join(captured.calls[0].args, " "), "set-buffer")
	assert.Contains(t, strings.Join(captured.calls[0].args, " "), "autopus-")
	// Call 2: paste-buffer
	assert.Contains(t, strings.Join(captured.calls[1].args, " "), "paste-buffer")
	assert.Contains(t, strings.Join(captured.calls[1].args, " "), "surface:7")
	// Call 3: delete-buffer
	assert.Contains(t, strings.Join(captured.calls[2].args, " "), "delete-buffer")
}

// TestCmuxAdapter_SendLongText_ShortText_SendCommandPath verifies short text (<500B)
// uses direct send command, not buffer path.
func TestCmuxAdapter_SendLongText_ShortText_SendCommandPath(t *testing.T) {
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	a := &CmuxAdapter{}
	shortText := strings.Repeat("B", 100)
	err := a.SendLongText(context.Background(), "surface:7", shortText)
	require.NoError(t, err)
	require.Len(t, captured.calls, 1, "short text should use single send call")
	combined := strings.Join(captured.calls[0].args, " ")
	assert.Contains(t, combined, "send")
	assert.Contains(t, combined, "--surface")
	assert.Contains(t, combined, "surface:7")
}

// TestCmuxAdapter_SendLongText_SetBufferFails_ChunkedFallback verifies fallback
// to chunked send when set-buffer fails.
func TestCmuxAdapter_SendLongText_SetBufferFails_ChunkedFallback(t *testing.T) {
	orig := execCommand
	var calls []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		combined := strings.Join(args, " ")
		calls = append(calls, combined)
		// Fail on set-buffer, succeed on everything else
		if strings.Contains(combined, "set-buffer") {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	defer func() { execCommand = orig }()

	a := &CmuxAdapter{}
	longText := strings.Repeat("C", 2000)
	err := a.SendLongText(context.Background(), "surface:7", longText)
	assert.NoError(t, err)
	// Should have called set-buffer (failed) then chunked send(s)
	require.True(t, len(calls) >= 2, "should attempt set-buffer then chunked fallback")
	// All fallback calls should be "send" commands
	for _, call := range calls[1:] {
		assert.Contains(t, call, "send", "fallback calls should use send")
	}
}

// TestCmuxAdapter_SendLongText_PasteBufferFails_ChunkedFallback verifies fallback
// to chunked send when paste-buffer fails (e.g., Codex ink TUI).
func TestCmuxAdapter_SendLongText_PasteBufferFails_ChunkedFallback(t *testing.T) {
	orig := execCommand
	var calls []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		combined := strings.Join(args, " ")
		calls = append(calls, combined)
		if strings.Contains(combined, "paste-buffer") {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	defer func() { execCommand = orig }()

	a := &CmuxAdapter{}
	longText := strings.Repeat("P", 5000)
	err := a.SendLongText(context.Background(), "surface:7", longText)
	assert.NoError(t, err)
	// Calls: set-buffer (ok) → paste-buffer (fail) → delete-buffer (cleanup) → chunked sends
	sendCalls := 0
	for _, call := range calls {
		if strings.Contains(call, "send") && !strings.Contains(call, "buffer") {
			sendCalls++
		}
	}
	assert.GreaterOrEqual(t, sendCalls, 2, "5000 bytes should need at least 2 chunked sends (3500 chunk size)")
}

// TestCmuxAdapter_sendChunked_SplitsCorrectly verifies chunked send splits
// text at 3500-byte boundaries.
func TestCmuxAdapter_sendChunked_SplitsCorrectly(t *testing.T) {
	orig := execCommand
	var sendPayloads []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		if len(args) >= 4 && args[0] == "send" {
			sendPayloads = append(sendPayloads, args[3])
		}
		return exec.Command("true")
	}
	defer func() { execCommand = orig }()

	a := &CmuxAdapter{}
	// 8000 bytes → 3 chunks: 3500 + 3500 + 1000
	text := strings.Repeat("X", 8000)
	err := a.sendChunked(context.Background(), "surface:7", text)
	require.NoError(t, err)
	require.Len(t, sendPayloads, 3, "8000 bytes / 3500 chunk = 3 chunks")
	assert.Len(t, sendPayloads[0], 3500)
	assert.Len(t, sendPayloads[1], 3500)
	assert.Len(t, sendPayloads[2], 1000)
	// Reassembled content matches original
	assert.Equal(t, text, sendPayloads[0]+sendPayloads[1]+sendPayloads[2])
}

// TestCmuxAdapter_SendLongText_UniqueBufferNames verifies different pane IDs produce
// different buffer names.
func TestCmuxAdapter_SendLongText_UniqueBufferNames(t *testing.T) {
	var allArgs [][]string
	orig := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		allArgs = append(allArgs, args)
		return exec.Command("true")
	}
	defer func() { execCommand = orig }()

	a := &CmuxAdapter{}
	longText := strings.Repeat("D", 2000)
	_ = a.SendLongText(context.Background(), "surface:7", longText)
	_ = a.SendLongText(context.Background(), "surface:8", longText)

	// Extract buffer names from set-buffer calls
	var bufNames []string
	for _, args := range allArgs {
		combined := strings.Join(args, " ")
		if strings.Contains(combined, "set-buffer") {
			for _, arg := range args {
				if strings.HasPrefix(arg, "autopus-") {
					bufNames = append(bufNames, arg)
				}
			}
		}
	}
	require.Len(t, bufNames, 2, "should have 2 set-buffer calls with buffer names")
	assert.NotEqual(t, bufNames[0], bufNames[1], "buffer names must be unique per pane")
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
