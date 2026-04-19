package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCmd_GlobalFlagsFlowIntoContext(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	var captured globalFlags
	root.AddCommand(&cobra.Command{
		Use: "capture",
		Run: func(cmd *cobra.Command, args []string) {
			captured = globalFlagsFromContext(cmd.Context())
		},
	})

	root.SetArgs([]string{"--auto", "--loop", "--multi", "--think", "--quality", "balanced", "--effort", "xhigh", "--task-created-mode", "enforce", "capture"})

	err := root.Execute()
	require.NoError(t, err)
	assert.True(t, captured.AutoMode)
	assert.True(t, captured.LoopMode)
	assert.True(t, captured.MultiMode)
	assert.True(t, captured.Think)
	assert.Equal(t, "balanced", captured.Quality)
	assert.Equal(t, "xhigh", captured.Effort)
	assert.Equal(t, "enforce", captured.TaskMode)
}

func TestRootCmd_UltrathinkImpliesThink(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	var captured globalFlags
	root.AddCommand(&cobra.Command{
		Use: "capture",
		Run: func(cmd *cobra.Command, args []string) {
			captured = globalFlagsFromContext(cmd.Context())
		},
	})

	root.SetArgs([]string{"--ultrathink", "capture"})

	err := root.Execute()
	require.NoError(t, err)
	assert.True(t, captured.UltraThink)
	assert.True(t, captured.Think)
}

func TestRootCmd_HelpShowsGlobalFlags(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--help"})

	err := root.Execute()
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "--auto")
	assert.Contains(t, output, "--loop")
	assert.Contains(t, output, "--multi")
	assert.Contains(t, output, "--think")
	assert.Contains(t, output, "--ultrathink")
	assert.Contains(t, output, "--quality")
	assert.Contains(t, output, "--effort")
	assert.Contains(t, output, "--task-created-mode")
}

func TestRootCmd_InvalidQualityFlagFails(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	root.SetArgs([]string{"--quality", "invalid", "version", "--short"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown quality preset "invalid"`)
}

func TestRootCmd_InvalidTaskCreatedModeFails(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	root.SetArgs([]string{"--task-created-mode", "block", "version", "--short"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid --task-created-mode "block"`)
}
