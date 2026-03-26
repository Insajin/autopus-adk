package terminal

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- R1: Terminal interface extension ---
// ReadScreen, PipePaneStart, PipePaneStop must be added to Terminal interface.

// TestInteractive_ReadScreen_CompileCheck verifies ReadScreen is part of the Terminal interface.
func TestInteractive_ReadScreen_CompileCheck(t *testing.T) {
	var _ Terminal = &CmuxAdapter{}
	var _ Terminal = &TmuxAdapter{}
	var _ Terminal = &PlainAdapter{}
}

// TestInteractive_PipePaneStart_CompileCheck verifies PipePaneStart is part of the Terminal interface.
func TestInteractive_PipePaneStart_CompileCheck(t *testing.T) {
	var _ Terminal = &CmuxAdapter{}
	var _ Terminal = &TmuxAdapter{}
	var _ Terminal = &PlainAdapter{}
}

// TestInteractive_PipePaneStop_CompileCheck verifies PipePaneStop is part of the Terminal interface.
func TestInteractive_PipePaneStop_CompileCheck(t *testing.T) {
	var _ Terminal = &CmuxAdapter{}
	var _ Terminal = &TmuxAdapter{}
	var _ Terminal = &PlainAdapter{}
}

// --- R2: CmuxAdapter ReadScreen / PipePaneStart / PipePaneStop ---

// TestInteractive_CmuxAdapter_ReadScreen_CallsReadScreenCmd verifies cmux read-screen --surface <ref>.
func TestInteractive_CmuxAdapter_ReadScreen_CallsReadScreenCmd(t *testing.T) {
	restore, captured := newCmuxMockV2("screen content here", nil)
	defer restore()

	a := &CmuxAdapter{}
	got, err := a.ReadScreen(context.Background(), "surface:7", ReadScreenOpts{})
	require.NoError(t, err)
	assert.Equal(t, "screen content here", got)
	combined := strings.Join(captured.lastArgs(), " ")
	assert.Contains(t, combined, "read-screen")
	assert.Contains(t, combined, "--surface")
	assert.Contains(t, combined, "surface:7")
}

// TestInteractive_CmuxAdapter_ReadScreen_WithScrollback tests scrollback option.
func TestInteractive_CmuxAdapter_ReadScreen_WithScrollback(t *testing.T) {
	restore, captured := newCmuxMockV2("scrollback content", nil)
	defer restore()

	a := &CmuxAdapter{}
	_, err := a.ReadScreen(context.Background(), "surface:7", ReadScreenOpts{Scrollback: true})
	require.NoError(t, err)
	combined := strings.Join(captured.lastArgs(), " ")
	assert.Contains(t, combined, "--scrollback")
}

// TestInteractive_CmuxAdapter_ReadScreen_WithLines tests --lines N option.
func TestInteractive_CmuxAdapter_ReadScreen_WithLines(t *testing.T) {
	restore, captured := newCmuxMockV2("last 50 lines", nil)
	defer restore()

	a := &CmuxAdapter{}
	_, err := a.ReadScreen(context.Background(), "surface:7", ReadScreenOpts{Lines: 50})
	require.NoError(t, err)
	combined := strings.Join(captured.lastArgs(), " ")
	assert.Contains(t, combined, "--lines")
	assert.Contains(t, combined, "50")
}

// TestInteractive_CmuxAdapter_PipePaneStart_CallsPipePaneCmd tests pipe-pane start.
func TestInteractive_CmuxAdapter_PipePaneStart_CallsPipePaneCmd(t *testing.T) {
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	a := &CmuxAdapter{}
	err := a.PipePaneStart(context.Background(), "surface:7", "/tmp/output.txt")
	require.NoError(t, err)
	combined := strings.Join(captured.lastArgs(), " ")
	assert.Contains(t, combined, "pipe-pane")
	assert.Contains(t, combined, "--surface")
	assert.Contains(t, combined, "surface:7")
	assert.Contains(t, combined, "cat >> '/tmp/output.txt'")
}

// TestInteractive_CmuxAdapter_PipePaneStop_CallsEmptyCommand tests pipe-pane stop.
func TestInteractive_CmuxAdapter_PipePaneStop_CallsEmptyCommand(t *testing.T) {
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	a := &CmuxAdapter{}
	err := a.PipePaneStop(context.Background(), "surface:7")
	require.NoError(t, err)
	combined := strings.Join(captured.lastArgs(), " ")
	assert.Contains(t, combined, "pipe-pane")
	assert.Contains(t, combined, "--command")
}

// --- R3: TmuxAdapter ReadScreen / PipePaneStart / PipePaneStop ---

// TestInteractive_TmuxAdapter_ReadScreen_CallsCapturePaneCmd tests tmux capture-pane.
func TestInteractive_TmuxAdapter_ReadScreen_CallsCapturePaneCmd(t *testing.T) {
	restore, captured := newTmuxMock()
	defer restore()

	a := &TmuxAdapter{}
	_, err := a.ReadScreen(context.Background(), "0", ReadScreenOpts{})
	require.NoError(t, err)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "capture-pane")
	assert.Contains(t, combined, "-t")
	assert.Contains(t, combined, "-p")
}

// TestInteractive_TmuxAdapter_PipePaneStart_CallsPipePaneCmd tests tmux pipe-pane start.
func TestInteractive_TmuxAdapter_PipePaneStart_CallsPipePaneCmd(t *testing.T) {
	restore, captured := newTmuxMock()
	defer restore()

	a := &TmuxAdapter{}
	err := a.PipePaneStart(context.Background(), "0", "/tmp/out.txt")
	require.NoError(t, err)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "pipe-pane")
	assert.Contains(t, combined, "-t")
	assert.Contains(t, combined, "-O")
}

// TestInteractive_TmuxAdapter_PipePaneStop_CallsPipePaneNoArgs tests tmux pipe-pane stop.
func TestInteractive_TmuxAdapter_PipePaneStop_CallsPipePaneNoArgs(t *testing.T) {
	restore, captured := newTmuxMock()
	defer restore()

	a := &TmuxAdapter{}
	err := a.PipePaneStop(context.Background(), "0")
	require.NoError(t, err)
	combined := strings.Join(captured.args, " ")
	assert.Contains(t, combined, "pipe-pane")
	assert.Contains(t, combined, "-t")
}

// --- R2/R3: Error paths ---

// TestInteractive_CmuxAdapter_ReadScreen_Error verifies error propagation.
func TestInteractive_CmuxAdapter_ReadScreen_Error(t *testing.T) {
	restore, _ := newCmuxMockV2("", fmt.Errorf("read-screen failed"))
	defer restore()

	a := &CmuxAdapter{}
	_, err := a.ReadScreen(context.Background(), "surface:7", ReadScreenOpts{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-screen")
}

// TestInteractive_CmuxAdapter_PipePaneStart_Error verifies error propagation.
func TestInteractive_CmuxAdapter_PipePaneStart_Error(t *testing.T) {
	restore, _ := newCmuxMockV2("", fmt.Errorf("pipe-pane failed"))
	defer restore()

	a := &CmuxAdapter{}
	err := a.PipePaneStart(context.Background(), "surface:7", "/tmp/out.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pipe-pane start")
}

// TestInteractive_CmuxAdapter_PipePaneStop_Error verifies error propagation.
func TestInteractive_CmuxAdapter_PipePaneStop_Error(t *testing.T) {
	restore, _ := newCmuxMockV2("", fmt.Errorf("pipe-pane stop failed"))
	defer restore()

	a := &CmuxAdapter{}
	err := a.PipePaneStop(context.Background(), "surface:7")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pipe-pane stop")
}

// TestInteractive_CmuxAdapter_ReadScreen_InvalidPaneID verifies validation error.
func TestInteractive_CmuxAdapter_ReadScreen_InvalidPaneID(t *testing.T) {
	a := &CmuxAdapter{}
	_, err := a.ReadScreen(context.Background(), "", ReadScreenOpts{})
	assert.Error(t, err)
}

// TestInteractive_CmuxAdapter_PipePaneStart_InvalidPaneID verifies validation error.
func TestInteractive_CmuxAdapter_PipePaneStart_InvalidPaneID(t *testing.T) {
	a := &CmuxAdapter{}
	err := a.PipePaneStart(context.Background(), "", "/tmp/out.txt")
	assert.Error(t, err)
}

// TestInteractive_CmuxAdapter_PipePaneStop_InvalidPaneID verifies validation error.
func TestInteractive_CmuxAdapter_PipePaneStop_InvalidPaneID(t *testing.T) {
	a := &CmuxAdapter{}
	err := a.PipePaneStop(context.Background(), "")
	assert.Error(t, err)
}

// TestInteractive_TmuxAdapter_ReadScreen_Error verifies error propagation.
func TestInteractive_TmuxAdapter_ReadScreen_Error(t *testing.T) {
	restore := newTmuxErrorMock()
	defer restore()

	a := &TmuxAdapter{}
	_, err := a.ReadScreen(context.Background(), "0", ReadScreenOpts{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "capture-pane")
}

// TestInteractive_TmuxAdapter_PipePaneStart_Error verifies error propagation.
func TestInteractive_TmuxAdapter_PipePaneStart_Error(t *testing.T) {
	restore := newTmuxErrorMock()
	defer restore()

	a := &TmuxAdapter{}
	err := a.PipePaneStart(context.Background(), "0", "/tmp/out.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pipe-pane start")
}

// TestInteractive_TmuxAdapter_PipePaneStop_Error verifies error propagation.
func TestInteractive_TmuxAdapter_PipePaneStop_Error(t *testing.T) {
	restore := newTmuxErrorMock()
	defer restore()

	a := &TmuxAdapter{}
	err := a.PipePaneStop(context.Background(), "0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pipe-pane stop")
}

// TestInteractive_TmuxAdapter_ReadScreen_InvalidPaneID verifies validation error.
func TestInteractive_TmuxAdapter_ReadScreen_InvalidPaneID(t *testing.T) {
	a := &TmuxAdapter{}
	_, err := a.ReadScreen(context.Background(), "", ReadScreenOpts{})
	assert.Error(t, err)
}

// TestInteractive_TmuxAdapter_PipePaneStart_InvalidPaneID verifies validation error.
func TestInteractive_TmuxAdapter_PipePaneStart_InvalidPaneID(t *testing.T) {
	a := &TmuxAdapter{}
	err := a.PipePaneStart(context.Background(), "", "/tmp/out.txt")
	assert.Error(t, err)
}

// TestInteractive_TmuxAdapter_PipePaneStop_InvalidPaneID verifies validation error.
func TestInteractive_TmuxAdapter_PipePaneStop_InvalidPaneID(t *testing.T) {
	a := &TmuxAdapter{}
	err := a.PipePaneStop(context.Background(), "")
	assert.Error(t, err)
}

// --- R4: PlainAdapter no-op ---

// TestInteractive_PlainAdapter_ReadScreen_ReturnsEmpty tests plain adapter returns empty.
func TestInteractive_PlainAdapter_ReadScreen_ReturnsEmpty(t *testing.T) {
	a := &PlainAdapter{}
	got, err := a.ReadScreen(context.Background(), "", ReadScreenOpts{})
	require.NoError(t, err)
	assert.Empty(t, got)
}

// TestInteractive_PlainAdapter_PipePaneStart_ReturnsNil tests plain adapter no-op.
func TestInteractive_PlainAdapter_PipePaneStart_ReturnsNil(t *testing.T) {
	a := &PlainAdapter{}
	err := a.PipePaneStart(context.Background(), "", "/tmp/out.txt")
	require.NoError(t, err)
}

// TestInteractive_PlainAdapter_PipePaneStop_ReturnsNil tests plain adapter no-op.
func TestInteractive_PlainAdapter_PipePaneStop_ReturnsNil(t *testing.T) {
	a := &PlainAdapter{}
	err := a.PipePaneStop(context.Background(), "")
	require.NoError(t, err)
}
