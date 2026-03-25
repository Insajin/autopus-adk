package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setPlainTerminalPath sets PATH to an empty directory so DetectTerminal() falls back to PlainAdapter.
// Returns a restore function. Must not be used with t.Parallel() as it modifies PATH.
func setPlainTerminalPath(t *testing.T) {
	t.Helper()
	// Use an empty temp dir so neither cmux nor tmux can be found.
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)
}

// TestTerminalDetectCmd verifies the detect subcommand runs and outputs a valid adapter name.
func TestTerminalDetectCmd(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"terminal", "detect"})

	err := root.Execute()
	require.NoError(t, err)

	output := strings.TrimSpace(buf.String())
	validNames := []string{"cmux", "tmux", "plain"}
	assert.Contains(t, validNames, output,
		"detect should output one of %v, got %q", validNames, output)
}

// TestTerminalCmd_NoSubcommand verifies that running `auto terminal` without a
// subcommand prints usage/help text.
func TestTerminalCmd_NoSubcommand(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"terminal"})

	err := root.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "detect", "help should list detect subcommand")
	assert.Contains(t, output, "workspace", "help should list workspace subcommand")
	assert.Contains(t, output, "split", "help should list split subcommand")
	assert.Contains(t, output, "send", "help should list send subcommand")
	assert.Contains(t, output, "notify", "help should list notify subcommand")
}

// TestTerminalWorkspaceCreateCmd verifies workspace create accepts the right arguments.
func TestTerminalWorkspaceCreateCmd(t *testing.T) {
	t.Parallel()

	// Verify that missing name argument produces an error.
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"terminal", "workspace", "create"})

	err := root.Execute()
	require.Error(t, err, "should fail without a workspace name argument")
}

// TestTerminalSplitCmd_InvalidDirection verifies that invalid direction is rejected.
func TestTerminalSplitCmd_InvalidDirection(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"terminal", "split", "x"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid direction")
}

// TestParseDirection verifies direction parsing logic.
func TestParseDirection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		wantErr bool
	}{
		{"h", false},
		{"v", false},
		{"x", true},
		{"", true},
	}

	for _, tc := range tests {
		_, err := parseDirection(tc.input)
		if tc.wantErr {
			assert.Error(t, err, "parseDirection(%q) should error", tc.input)
		} else {
			assert.NoError(t, err, "parseDirection(%q) should not error", tc.input)
		}
	}
}

// TestTerminalWorkspaceCloseCmd_NoArgs verifies that close without a name argument errors.
func TestTerminalWorkspaceCloseCmd_NoArgs(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"terminal", "workspace", "close"})

	err := root.Execute()
	require.Error(t, err, "close without workspace name must return an error")
}

// TestTerminalSplitCmd_NoArgs verifies that split without a direction argument errors.
func TestTerminalSplitCmd_NoArgs(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"terminal", "split"})

	err := root.Execute()
	require.Error(t, err, "split without direction must return an error")
}

// TestTerminalSendCmd_NoArgs verifies that send without arguments errors.
func TestTerminalSendCmd_NoArgs(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"terminal", "send"})

	err := root.Execute()
	require.Error(t, err, "send without pane-id and command must return an error")
}

// TestTerminalNotifyCmd_NoArgs verifies that notify without a message argument errors.
func TestTerminalNotifyCmd_NoArgs(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"terminal", "notify"})

	err := root.Execute()
	require.Error(t, err, "notify without message must return an error")
}

// TestTerminalWorkspaceCreateCmd_PlainAdapter verifies workspace create succeeds with plain adapter.
// Note: uses t.Setenv to force plain adapter — cannot use t.Parallel().
func TestTerminalWorkspaceCreateCmd_PlainAdapter(t *testing.T) {
	setPlainTerminalPath(t)

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"terminal", "workspace", "create", "test-workspace"})

	err := root.Execute()
	require.NoError(t, err, "workspace create with plain adapter must succeed")
	assert.Contains(t, buf.String(), "test-workspace", "output must mention workspace name")
}

// TestTerminalWorkspaceCloseCmd_PlainAdapter verifies workspace close succeeds with plain adapter.
// Note: uses t.Setenv to force plain adapter — cannot use t.Parallel().
func TestTerminalWorkspaceCloseCmd_PlainAdapter(t *testing.T) {
	setPlainTerminalPath(t)

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"terminal", "workspace", "close", "test-workspace"})

	err := root.Execute()
	require.NoError(t, err, "workspace close with plain adapter must succeed")
	assert.Contains(t, buf.String(), "test-workspace", "output must mention workspace name")
}

// TestTerminalSplitCmd_PlainAdapter verifies split succeeds with plain adapter.
// Note: uses t.Setenv to force plain adapter — cannot use t.Parallel().
func TestTerminalSplitCmd_PlainAdapter(t *testing.T) {
	setPlainTerminalPath(t)

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"terminal", "split", "h"})

	err := root.Execute()
	require.NoError(t, err, "split with plain adapter must succeed")
}

// TestTerminalSendCmd_PlainAdapter verifies send succeeds with plain adapter.
// Note: uses t.Setenv to force plain adapter — cannot use t.Parallel().
func TestTerminalSendCmd_PlainAdapter(t *testing.T) {
	setPlainTerminalPath(t)

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"terminal", "send", "pane-0", "echo hello"})

	err := root.Execute()
	require.NoError(t, err, "send with plain adapter must succeed")
	assert.Contains(t, buf.String(), "pane-0", "output must mention pane ID")
}

// TestTerminalNotifyCmd_PlainAdapter verifies notify succeeds with plain adapter.
// Note: uses t.Setenv to force plain adapter — cannot use t.Parallel().
func TestTerminalNotifyCmd_PlainAdapter(t *testing.T) {
	setPlainTerminalPath(t)

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"terminal", "notify", "build done"})

	err := root.Execute()
	require.NoError(t, err, "notify with plain adapter must succeed")
	assert.Contains(t, buf.String(), "notified", "output must confirm notification")
}

// TestParseDirection_TableDriven is a comprehensive table-driven test for parseDirection.
func TestParseDirection_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{"horizontal lowercase", "h", false, ""},
		{"vertical lowercase", "v", false, ""},
		{"uppercase H", "H", true, "invalid direction"},
		{"uppercase V", "V", true, "invalid direction"},
		{"number", "1", true, "invalid direction"},
		{"empty string", "", true, "invalid direction"},
		{"multi char", "hv", true, "invalid direction"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseDirection(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errMsg != "" {
					assert.Contains(t, err.Error(), tc.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
