package terminal

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInteractive_CmuxAdapter_ReadScreen_UsesContextCommand(t *testing.T) {
	orig := execCommand
	origCtx := execCommandContext
	defer func() {
		execCommand = orig
		execCommandContext = origCtx
	}()

	plainCalled := false
	contextCalled := false
	execCommand = func(_ string, _ ...string) *exec.Cmd {
		plainCalled = true
		return exec.Command("false")
	}
	execCommandContext = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		contextCalled = true
		return exec.Command("printf", "%s", "ctx screen")
	}

	a := &CmuxAdapter{}
	got, err := a.ReadScreen(context.Background(), "surface:7", ReadScreenOpts{})
	require.NoError(t, err)
	assert.Equal(t, "ctx screen", got)
	assert.True(t, contextCalled)
	assert.False(t, plainCalled)
}

func TestInteractive_TmuxAdapter_ReadScreen_UsesContextCommand(t *testing.T) {
	orig := execCommand
	origCtx := execCommandContext
	defer func() {
		execCommand = orig
		execCommandContext = origCtx
	}()

	plainCalled := false
	contextCalled := false
	execCommand = func(_ string, _ ...string) *exec.Cmd {
		plainCalled = true
		return exec.Command("false")
	}
	execCommandContext = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		contextCalled = true
		return exec.Command("printf", "%s", "ctx tmux")
	}

	a := &TmuxAdapter{}
	got, err := a.ReadScreen(context.Background(), "0", ReadScreenOpts{})
	require.NoError(t, err)
	assert.Equal(t, "ctx tmux", got)
	assert.True(t, contextCalled)
	assert.False(t, plainCalled)
}

func TestCmuxAdapter_SendCommandUsesContextCommand(t *testing.T) {
	orig := execCommand
	origCtx := execCommandContext
	defer func() {
		execCommand = orig
		execCommandContext = origCtx
	}()

	plainCalled := false
	contextCalls := 0
	execCommand = func(_ string, _ ...string) *exec.Cmd {
		plainCalled = true
		return exec.Command("false")
	}
	execCommandContext = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		contextCalls++
		return exec.Command("true")
	}

	err := (&CmuxAdapter{}).SendCommand(context.Background(), "surface:7", "\n")
	require.NoError(t, err)
	assert.Equal(t, 1, contextCalls)
	assert.False(t, plainCalled)
}

func TestCmuxAdapter_SendLongTextUsesContextCommands(t *testing.T) {
	orig := execCommand
	origCtx := execCommandContext
	defer func() {
		execCommand = orig
		execCommandContext = origCtx
	}()

	plainCalled := false
	contextCalls := 0
	execCommand = func(_ string, _ ...string) *exec.Cmd {
		plainCalled = true
		return exec.Command("false")
	}
	execCommandContext = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		contextCalls++
		return exec.Command("true")
	}

	err := (&CmuxAdapter{}).SendLongText(context.Background(), "surface:7", "launch")
	require.NoError(t, err)
	assert.Equal(t, 3, contextCalls)
	assert.False(t, plainCalled)
}
