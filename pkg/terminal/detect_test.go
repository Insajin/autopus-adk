package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetectTerminal_InactiveCmuxInstallationIgnored verifies that installing cmux
// alone does not imply that the current process can target a cmux workspace.
// Note: cannot use t.Parallel() — this test mutates the package-level isInstalled variable.
func TestDetectTerminal_InactiveCmuxInstallationIgnored(t *testing.T) {
	clearMuxContext(t)

	// Replace isInstalled to simulate cmux being installed, tmux not.
	orig := isInstalled
	t.Cleanup(func() { isInstalled = orig })

	isInstalled = func(binary string) bool {
		return binary == "cmux"
	}

	term := DetectTerminal()
	require.NotNil(t, term, "DetectTerminal must always return a non-nil terminal")
	assert.Equal(t, "plain", term.Name(), "inactive cmux installation must not be treated as the current multiplexer")
}

// TestDetectTerminal_ActiveCmuxContextSelected verifies that inherited cmux
// context identifies the workspace that pane operations can safely target.
// Note: cannot use t.Parallel() — this test mutates the package-level isInstalled variable.
func TestDetectTerminal_ActiveCmuxContextSelected(t *testing.T) {
	for _, key := range []string{
		"CMUX_SOCKET_PATH",
		"CMUX_WORKSPACE_ID",
		"CMUX_SURFACE_ID",
		"CMUX_PANE_ID",
	} {
		t.Run(key, func(t *testing.T) {
			clearMuxContext(t)
			t.Setenv(key, "active-context")

			orig := isInstalled
			t.Cleanup(func() { isInstalled = orig })
			isInstalled = func(binary string) bool {
				return binary == "cmux"
			}

			term := DetectTerminal()
			require.NotNil(t, term)
			assert.Equal(t, "cmux", term.Name(), "active cmux context must select the cmux adapter")
		})
	}
}

// TestDetectTerminal_InactiveTmuxInstallationIgnored verifies that installing
// tmux alone does not make pane operations target an unrelated server session.
// Note: cannot use t.Parallel() — this test mutates the package-level isInstalled variable.
func TestDetectTerminal_InactiveTmuxInstallationIgnored(t *testing.T) {
	clearMuxContext(t)

	orig := isInstalled
	t.Cleanup(func() { isInstalled = orig })

	isInstalled = func(binary string) bool {
		return binary == "tmux"
	}

	term := DetectTerminal()
	require.NotNil(t, term, "DetectTerminal must always return a non-nil terminal")
	assert.Equal(t, "plain", term.Name(), "inactive tmux installation must not be treated as the current multiplexer")
}

// TestDetectTerminal_InactiveMuxInstallationsReturnPlain verifies that pane
// routing does not select either multiplexer without an inherited context.
func TestDetectTerminal_InactiveMuxInstallationsReturnPlain(t *testing.T) {
	clearMuxContext(t)

	orig := isInstalled
	t.Cleanup(func() { isInstalled = orig })
	isInstalled = func(binary string) bool {
		return binary == "cmux" || binary == "tmux"
	}

	term := DetectTerminal()
	require.NotNil(t, term)
	assert.Equal(t, "plain", term.Name())
}

// TestDetectInstalledTerminal_ManagementPriorityPreservesInstalledMux verifies
// the explicit management contract independently from active pane detection.
func TestDetectInstalledTerminal_ManagementPriorityPreservesInstalledMux(t *testing.T) {
	tests := []struct {
		name      string
		installed map[string]bool
		want      string
	}{
		{
			name:      "cmux preferred when both are installed",
			installed: map[string]bool{"cmux": true, "tmux": true},
			want:      "cmux",
		},
		{
			name:      "tmux fallback",
			installed: map[string]bool{"tmux": true},
			want:      "tmux",
		},
		{
			name:      "plain fallback",
			installed: map[string]bool{},
			want:      "plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := isInstalled
			t.Cleanup(func() { isInstalled = orig })
			isInstalled = func(binary string) bool {
				return tt.installed[binary]
			}

			term := DetectInstalledTerminal()
			require.NotNil(t, term)
			assert.Equal(t, tt.want, term.Name())
		})
	}
}

// TestDetectTerminal_PlainFallback verifies that DetectTerminal returns a plain adapter
// when neither cmux nor tmux is installed.
// Note: cannot use t.Parallel() — this test mutates the package-level isInstalled variable.
func TestDetectTerminal_PlainFallback(t *testing.T) {
	clearMuxContext(t)

	orig := isInstalled
	t.Cleanup(func() { isInstalled = orig })

	isInstalled = func(_ string) bool {
		return false
	}

	term := DetectTerminal()
	require.NotNil(t, term, "DetectTerminal must return a non-nil terminal (plain fallback)")
	assert.Equal(t, "plain", term.Name(), "must return plain adapter when no multiplexer is installed")
}

// TestDetectTerminal_ActiveTmuxPreferredWhenBothInstalled verifies that active
// session identity wins over the static cmux-first installation priority.
func TestDetectTerminal_ActiveTmuxPreferredWhenBothInstalled(t *testing.T) {
	// Given: both multiplexers are installed, but this process belongs to tmux.
	clearMuxContext(t)
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:7")
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	orig := isInstalled
	t.Cleanup(func() { isInstalled = orig })
	isInstalled = func(binary string) bool {
		return binary == "cmux" || binary == "tmux"
	}

	// When: the terminal adapter is detected.
	term := DetectTerminal()

	// Then: panes must be opened in the active tmux session, not in cmux.
	require.NotNil(t, term)
	assert.Equal(t, "tmux", term.Name(), "active TMUX must beat cmux installation priority")
}

func clearMuxContext(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"TMUX",
		"CMUX_SOCKET_PATH",
		"CMUX_WORKSPACE_ID",
		"CMUX_SURFACE_ID",
		"CMUX_PANE_ID",
	} {
		t.Setenv(key, "")
	}
}
