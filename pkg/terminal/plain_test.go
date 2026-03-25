package terminal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlainAdapter_Name verifies Name returns "plain".
func TestPlainAdapter_Name(t *testing.T) {
	t.Parallel()

	a := &PlainAdapter{}
	assert.Equal(t, "plain", a.Name())
}

// TestPlainAdapter_CreateWorkspace_NoError verifies CreateWorkspace returns nil.
func TestPlainAdapter_CreateWorkspace_NoError(t *testing.T) {
	t.Parallel()

	a := &PlainAdapter{}
	err := a.CreateWorkspace(context.Background(), "workspace")
	require.NoError(t, err)
}

// TestPlainAdapter_SplitPane_NoError verifies SplitPane returns nil error.
func TestPlainAdapter_SplitPane_NoError(t *testing.T) {
	t.Parallel()

	a := &PlainAdapter{}
	_, err := a.SplitPane(context.Background(), Horizontal)
	require.NoError(t, err)
}

// TestPlainAdapter_SendCommand_NoError verifies SendCommand returns nil.
func TestPlainAdapter_SendCommand_NoError(t *testing.T) {
	t.Parallel()

	a := &PlainAdapter{}
	err := a.SendCommand(context.Background(), "0", "echo hello")
	require.NoError(t, err)
}

// TestPlainAdapter_Close_NoError verifies Close returns nil.
func TestPlainAdapter_Close_NoError(t *testing.T) {
	t.Parallel()

	a := &PlainAdapter{}
	err := a.Close(context.Background(), "workspace")
	require.NoError(t, err)
}

// TestPlainAdapter_Notify_NoError verifies Notify returns nil.
func TestPlainAdapter_Notify_NoError(t *testing.T) {
	t.Parallel()

	a := &PlainAdapter{}
	err := a.Notify(context.Background(), "hello")
	require.NoError(t, err)
}
