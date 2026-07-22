package terminal

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCmuxAdapter_SplitPaneUsesContextCommand(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	original := execCommand
	originalContext := execCommandContext
	defer func() {
		execCommand = original
		execCommandContext = originalContext
	}()

	plainCalled := false
	contextCalled := false
	execCommand = func(_ string, _ ...string) *exec.Cmd {
		plainCalled = true
		return exec.Command("false")
	}
	execCommandContext = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		contextCalled = true
		return exec.Command("printf", "%s", "OK surface:7 workspace:1")
	}

	paneID, err := (&CmuxAdapter{}).SplitPane(context.Background(), Horizontal)

	require.NoError(t, err)
	assert.Equal(t, PaneID("surface:7"), paneID)
	assert.True(t, contextCalled)
	assert.False(t, plainCalled)
}

func TestTmuxAdapter_SplitPaneUsesContextCommand(t *testing.T) {
	original := execCommand
	originalContext := execCommandContext
	defer func() {
		execCommand = original
		execCommandContext = originalContext
	}()

	plainCalled := false
	contextCalled := false
	execCommand = func(_ string, _ ...string) *exec.Cmd {
		plainCalled = true
		return exec.Command("false")
	}
	execCommandContext = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		contextCalled = true
		return exec.Command("printf", "%s", "%7\n")
	}

	paneID, err := (&TmuxAdapter{}).SplitPane(context.Background(), Horizontal)

	require.NoError(t, err)
	assert.Equal(t, PaneID("%7"), paneID)
	assert.True(t, contextCalled)
	assert.False(t, plainCalled)
}
