package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPermissionDetectCmd_PlainOutput verifies the plain text output of
// "auto permission detect" returns either "bypass" or "safe".
func TestPermissionDetectCmd_PlainOutput(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"permission", "detect"})
	require.NoError(t, cmd.Execute())

	output := out.String()
	assert.Contains(t, []string{"bypass\n", "safe\n"}, output,
		"permission detect must output 'bypass' or 'safe'")
}

// TestPermissionDetectCmd_JSONOutput verifies --json flag produces valid JSON
// with required fields: mode, parent_pid, flag_found.
func TestPermissionDetectCmd_JSONOutput(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"permission", "detect", "--json"})
	require.NoError(t, cmd.Execute())

	var result struct {
		SchemaVersion string                 `json:"schema_version"`
		Status        string                 `json:"status"`
		Data          map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &result),
		"--json output must be valid JSON")

	assert.Equal(t, "1.0.0", result.SchemaVersion)
	assert.Equal(t, "ok", result.Status)
	assert.Contains(t, result.Data, "mode", "JSON data must contain 'mode' field")
	assert.Contains(t, result.Data, "parent_pid", "JSON data must contain 'parent_pid' field")
	assert.Contains(t, result.Data, "flag_found", "JSON data must contain 'flag_found' field")

	mode, ok := result.Data["mode"].(string)
	require.True(t, ok, "mode must be a string")
	assert.Contains(t, []string{"bypass", "safe"}, mode)
}

// TestPermissionDetectCmd_EnvBypass verifies env override produces bypass in JSON.
func TestPermissionDetectCmd_EnvBypass(t *testing.T) {
	t.Setenv("AUTOPUS_PERMISSION_MODE", "bypass")

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"permission", "detect", "--json"})
	require.NoError(t, cmd.Execute())

	var result struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	assert.Equal(t, "bypass", result.Data["mode"])
}

// TestPermissionDetectCmd_EnvSafe verifies env override produces safe in JSON.
func TestPermissionDetectCmd_EnvSafe(t *testing.T) {
	t.Setenv("AUTOPUS_PERMISSION_MODE", "safe")

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"permission", "detect", "--json"})
	require.NoError(t, cmd.Execute())

	var result struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	assert.Equal(t, "safe", result.Data["mode"])
}

// TestPermissionCmd_NoSubcommand verifies "auto permission" with no subcommand
// shows help without error.
func TestPermissionCmd_NoSubcommand(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"permission"})
	// Cobra shows usage when no subcommand is given; should not error.
	err := cmd.Execute()
	require.NoError(t, err)
}
