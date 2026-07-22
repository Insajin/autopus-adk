package terminal

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCmuxAdapter_SendLongText_LongText_BufferPath verifies long text (>=500B)
// uses set-buffer/paste-buffer and clears the reusable buffer instead of send.
func TestCmuxAdapter_SendLongText_LongText_BufferPath(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	a := &CmuxAdapter{}
	longText := strings.Repeat("A", 2000)
	err := a.SendLongText(context.Background(), "surface:7", longText)
	require.NoError(t, err)
	require.Len(t, captured.calls, 3, "long text should use buffer path (3 calls)")
	assert.Contains(t, strings.Join(captured.calls[0].args, " "), "set-buffer")
	assert.Contains(t, strings.Join(captured.calls[0].args, " "), "autopus-")
	assert.Contains(t, strings.Join(captured.calls[1].args, " "), "paste-buffer")
	assert.Contains(t, strings.Join(captured.calls[1].args, " "), "surface:7")
	assert.Equal(t, []string{"set-buffer", "--name", cmuxInputBufferName, "--", ""}, captured.calls[2].args)
}

// TestCmuxAdapter_SendLongText_ShortText_BufferPath verifies short text also uses
// buffer paste so cmux input does not pass through the active IME state.
func TestCmuxAdapter_SendLongText_ShortText_BufferPath(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	a := &CmuxAdapter{}
	shortText := strings.Repeat("B", 100)
	err := a.SendLongText(context.Background(), "surface:7", shortText)
	require.NoError(t, err)
	require.Len(t, captured.calls, 3, "short text should use buffer path (3 calls)")
	assert.Contains(t, strings.Join(captured.calls[0].args, " "), "set-buffer")
	assert.Contains(t, strings.Join(captured.calls[1].args, " "), "paste-buffer")
	assert.Contains(t, strings.Join(captured.calls[1].args, " "), "surface:7")
	assert.Equal(t, []string{"set-buffer", "--name", cmuxInputBufferName, "--", ""}, captured.calls[2].args)
}

// TestCmuxAdapter_SendLongText_SetBufferFails_ChunkedFallback verifies fallback
// to chunked send when set-buffer fails.
func TestCmuxAdapter_SendLongText_SetBufferFails_ChunkedFallback(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	orig := execCommand
	origCtx := execCommandContext
	var calls []string
	buildCmd := func(name string, args ...string) *exec.Cmd {
		combined := strings.Join(args, " ")
		calls = append(calls, combined)
		if strings.Contains(combined, "set-buffer") {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	execCommand = buildCmd
	execCommandContext = func(_ context.Context, name string, args ...string) *exec.Cmd { return buildCmd(name, args...) }
	defer func() {
		execCommand = orig
		execCommandContext = origCtx
	}()

	a := &CmuxAdapter{}
	longText := strings.Repeat("C", 2000)
	err := a.SendLongText(context.Background(), "surface:7", longText)
	assert.NoError(t, err)
	require.True(t, len(calls) >= 2, "should attempt set-buffer then chunked fallback")
	for _, call := range calls[1:] {
		assert.Contains(t, call, "send", "fallback calls should use send")
	}
}

// TestCmuxAdapter_SendLongText_PasteBufferFails_ChunkedFallback verifies fallback
// to chunked send when paste-buffer fails (e.g., Codex ink TUI).
func TestCmuxAdapter_SendLongText_PasteBufferFails_ChunkedFallback(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	orig := execCommand
	origCtx := execCommandContext
	var calls []string
	buildCmd := func(name string, args ...string) *exec.Cmd {
		combined := strings.Join(args, " ")
		calls = append(calls, combined)
		if strings.Contains(combined, "paste-buffer") {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	execCommand = buildCmd
	execCommandContext = func(_ context.Context, name string, args ...string) *exec.Cmd { return buildCmd(name, args...) }
	defer func() {
		execCommand = orig
		execCommandContext = origCtx
	}()

	a := &CmuxAdapter{}
	longText := strings.Repeat("P", 5000)
	err := a.SendLongText(context.Background(), "surface:7", longText)
	assert.NoError(t, err)

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
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	orig := execCommand
	origCtx := execCommandContext
	var sendPayloads []string
	buildCmd := func(name string, args ...string) *exec.Cmd {
		if len(args) >= 2 && args[0] == "send" {
			sendPayloads = append(sendPayloads, args[len(args)-1])
		}
		return exec.Command("true")
	}
	execCommand = buildCmd
	execCommandContext = func(_ context.Context, name string, args ...string) *exec.Cmd { return buildCmd(name, args...) }
	defer func() {
		execCommand = orig
		execCommandContext = origCtx
	}()

	a := &CmuxAdapter{}
	text := strings.Repeat("X", 8000)
	err := a.sendChunked(context.Background(), "surface:7", text)
	require.NoError(t, err)
	require.Len(t, sendPayloads, 3, "8000 bytes / 3500 chunk = 3 chunks")
	assert.Len(t, sendPayloads[0], 3500)
	assert.Len(t, sendPayloads[1], 3500)
	assert.Len(t, sendPayloads[2], 1000)
	assert.Equal(t, text, sendPayloads[0]+sendPayloads[1]+sendPayloads[2])
}

// TestCmuxAdapter_SendLongText_ReusesBufferAcrossPanes verifies pane churn does
// not create more cmux buffers.
func TestCmuxAdapter_SendLongText_ReusesBufferAcrossPanes(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	var allArgs [][]string
	orig := execCommand
	origCtx := execCommandContext
	buildCmd := func(name string, args ...string) *exec.Cmd {
		allArgs = append(allArgs, args)
		return exec.Command("true")
	}
	execCommand = buildCmd
	execCommandContext = func(_ context.Context, name string, args ...string) *exec.Cmd { return buildCmd(name, args...) }
	defer func() {
		execCommand = orig
		execCommandContext = origCtx
	}()

	a := &CmuxAdapter{}
	longText := strings.Repeat("D", 2000)
	_ = a.SendLongText(context.Background(), "surface:7", longText)
	_ = a.SendLongText(context.Background(), "surface:8", longText)

	var bufNames []string
	for _, args := range allArgs {
		combined := strings.Join(args, " ")
		if strings.Contains(combined, "set-buffer") && args[len(args)-1] != "" {
			for _, arg := range args {
				if strings.HasPrefix(arg, "autopus-") {
					bufNames = append(bufNames, arg)
				}
			}
		}
	}
	require.Len(t, bufNames, 2, "should have 2 set-buffer calls with buffer names")
	assert.Equal(t, bufNames[0], bufNames[1], "all panes must reuse the bounded buffer")
	assert.Equal(t, cmuxInputBufferName, bufNames[0])
}
