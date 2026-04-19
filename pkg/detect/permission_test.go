package detect

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubProcessTreeCheck swaps the package-level process tree checker for the
// duration of a test, so DetectPermissionMode() does not consult the real
// parent process. Required when tests run inside a Claude Code session
// launched with --dangerously-skip-permissions — the real checker would
// report bypass regardless of the scenario under test.
func stubProcessTreeCheck(t *testing.T, fn func() (bool, int, error)) {
	t.Helper()
	original := checkProcessTreeFn
	checkProcessTreeFn = fn
	t.Cleanup(func() { checkProcessTreeFn = original })
}

// TestDetectPermissionMode_EnvOverride_ReturnsBypass verifies that
// AUTOPUS_PERMISSION_MODE=bypass skips process check and returns bypass.
func TestDetectPermissionMode_EnvOverride_ReturnsBypass(t *testing.T) {
	t.Setenv("AUTOPUS_PERMISSION_MODE", "bypass")

	result := DetectPermissionMode()
	assert.Equal(t, "bypass", result.Mode)
	assert.False(t, result.FlagFound, "env override should not set FlagFound")
}

// TestDetectPermissionMode_EnvOverride_ReturnsSafe verifies that
// AUTOPUS_PERMISSION_MODE=safe skips process check and returns safe.
func TestDetectPermissionMode_EnvOverride_ReturnsSafe(t *testing.T) {
	t.Setenv("AUTOPUS_PERMISSION_MODE", "safe")

	result := DetectPermissionMode()
	assert.Equal(t, "safe", result.Mode)
	assert.False(t, result.FlagFound, "env override should not set FlagFound")
}

// TestDetectPermissionMode_InvalidEnv_FallsBackToProcessCheck verifies that
// an invalid AUTOPUS_PERMISSION_MODE value is ignored and process check runs.
func TestDetectPermissionMode_InvalidEnv_FallsBackToProcessCheck(t *testing.T) {
	tests := []struct {
		name   string
		envVal string
	}{
		{"random string", "invalid-value"},
		{"empty string", ""},
		{"uppercase BYPASS", "BYPASS"},
		{"numeric", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AUTOPUS_PERMISSION_MODE", tt.envVal)
			// Clear cmux env to prevent cmux heuristic from returning bypass
			t.Setenv("CMUX_CLAUDE_PID", "")
			// Stub out the real process tree walker so the assertion is
			// deterministic even when the host session has the flag set.
			stubProcessTreeCheck(t, func() (bool, int, error) {
				return false, 0, nil
			})

			result := DetectPermissionMode()
			// When env is invalid, falls back to process check.
			// Stubbed checker reports no flag → expect "safe" as fail-safe.
			assert.Equal(t, "safe", result.Mode)
		})
	}
}

// TestDetectPermissionMode_ProcessCheckFails_ReturnsSafe verifies fail-safe
// behavior: when process tree inspection fails, return "safe".
func TestDetectPermissionMode_ProcessCheckFails_ReturnsSafe(t *testing.T) {
	// Ensure env override is not set so process check path is taken.
	t.Setenv("AUTOPUS_PERMISSION_MODE", "")
	// Clear cmux env to prevent cmux heuristic from returning bypass
	t.Setenv("CMUX_CLAUDE_PID", "")
	// Simulate process tree inspection error — detectPermissionModeWith
	// should fall back to safe when the checker returns err.
	stubProcessTreeCheck(t, func() (bool, int, error) {
		return false, 0, assert.AnError
	})

	result := DetectPermissionMode()
	assert.Equal(t, "safe", result.Mode)
}

// TestDetectPermissionMode_FlagFound_ReturnsBypass verifies that when the
// --dangerously-skip-permissions flag is found in parent process tree,
// mode is "bypass" and FlagFound is true.
func TestDetectPermissionMode_FlagFound_ReturnsBypass(t *testing.T) {
	t.Parallel()

	// Use the injectable checker to simulate flag found in process tree.
	result := detectPermissionModeWith(func() (bool, int, error) {
		return true, 12345, nil
	})
	assert.Equal(t, "bypass", result.Mode)
	assert.True(t, result.FlagFound)
	assert.Equal(t, 12345, result.ParentPID)
}

// TestDetectPermissionMode_FlagNotFound_ReturnsSafe verifies that when the
// flag is not found in the process tree, mode is "safe".
func TestDetectPermissionMode_FlagNotFound_ReturnsSafe(t *testing.T) {
	t.Parallel()

	result := detectPermissionModeWith(func() (bool, int, error) {
		return false, 0, nil
	})
	assert.Equal(t, "safe", result.Mode)
	assert.False(t, result.FlagFound)
}

// TestPermissionResult_JSON verifies JSON marshaling matches --json output spec.
func TestPermissionResult_JSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		result   PermissionResult
		expected map[string]interface{}
	}{
		{
			"bypass mode",
			PermissionResult{Mode: "bypass", ParentPID: 42, FlagFound: true},
			map[string]interface{}{
				"mode":       "bypass",
				"parent_pid": float64(42),
				"flag_found": true,
			},
		},
		{
			"safe mode",
			PermissionResult{Mode: "safe", ParentPID: 0, FlagFound: false},
			map[string]interface{}{
				"mode":       "safe",
				"parent_pid": float64(0),
				"flag_found": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.result)
			require.NoError(t, err)

			var got map[string]interface{}
			require.NoError(t, json.Unmarshal(data, &got))

			assert.Equal(t, tt.expected["mode"], got["mode"])
			assert.Equal(t, tt.expected["parent_pid"], got["parent_pid"])
			assert.Equal(t, tt.expected["flag_found"], got["flag_found"])
		})
	}
}

// TestDetectPermissionMode_CmuxHeuristic_ReturnsBypass verifies that when
// CMUX_CLAUDE_PID is set and process tree has no flag, cmux heuristic kicks in.
func TestDetectPermissionMode_CmuxHeuristic_ReturnsBypass(t *testing.T) {
	t.Setenv("AUTOPUS_PERMISSION_MODE", "")
	t.Setenv("CMUX_CLAUDE_PID", "12345")

	result := DetectPermissionMode()
	assert.Equal(t, "bypass", result.Mode)
	assert.True(t, result.FlagFound)
}

// TestDetectPermissionMode_NoCmux_ReturnsSafe verifies that without CMUX_CLAUDE_PID
// and no flag in process tree, mode is "safe".
func TestDetectPermissionMode_NoCmux_ReturnsSafe(t *testing.T) {
	t.Setenv("AUTOPUS_PERMISSION_MODE", "")
	t.Setenv("CMUX_CLAUDE_PID", "")
	// Stub the process tree walker to simulate a clean environment
	// where no ancestor carries --dangerously-skip-permissions.
	stubProcessTreeCheck(t, func() (bool, int, error) {
		return false, 0, nil
	})

	result := DetectPermissionMode()
	assert.Equal(t, "safe", result.Mode)
}

// TestDetectPermissionModeWith_ErrorFallback verifies that when the process
// checker returns an error, the result falls back to "safe".
func TestDetectPermissionModeWith_ErrorFallback(t *testing.T) {
	t.Parallel()

	result := detectPermissionModeWith(func() (bool, int, error) {
		return false, 0, assert.AnError
	})
	assert.Equal(t, "safe", result.Mode)
	assert.False(t, result.FlagFound)
}
