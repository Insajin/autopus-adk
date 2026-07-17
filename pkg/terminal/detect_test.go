package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetectTerminal_CmuxAvailable verifies that DetectTerminal returns a cmux adapter
// when cmux is the only installed multiplexer.
// Note: cannot use t.Parallel() — this test mutates the package-level isInstalled variable.
func TestDetectTerminal_CmuxAvailable(t *testing.T) {
	// Replace isInstalled to simulate cmux being installed, tmux not.
	orig := isInstalled
	t.Cleanup(func() { isInstalled = orig })

	isInstalled = func(binary string) bool {
		return binary == "cmux"
	}

	term := DetectTerminal()
	require.NotNil(t, term, "DetectTerminal must return a non-nil terminal when cmux is installed")
	assert.Equal(t, "cmux", term.Name(), "must return cmux adapter when cmux is installed")
}

// TestDetectTerminal_TmuxFallback verifies that DetectTerminal returns a tmux adapter
// when only tmux is installed.
// Note: cannot use t.Parallel() — this test mutates the package-level isInstalled variable.
func TestDetectTerminal_TmuxFallback(t *testing.T) {
	orig := isInstalled
	t.Cleanup(func() { isInstalled = orig })

	isInstalled = func(binary string) bool {
		return binary == "tmux"
	}

	term := DetectTerminal()
	require.NotNil(t, term, "DetectTerminal must return a non-nil terminal when tmux is installed")
	assert.Equal(t, "tmux", term.Name(), "must return tmux adapter when tmux is installed and cmux is not")
}

// TestDetectTerminal_PlainFallback verifies that DetectTerminal returns a plain adapter
// when neither cmux nor tmux is installed.
// Note: cannot use t.Parallel() — this test mutates the package-level isInstalled variable.
func TestDetectTerminal_PlainFallback(t *testing.T) {
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
	for _, key := range []string{
		"CMUX_SOCKET_PATH",
		"CMUX_WORKSPACE_ID",
		"CMUX_SURFACE_ID",
		"CMUX_PANE_ID",
	} {
		t.Setenv(key, "")
	}
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
