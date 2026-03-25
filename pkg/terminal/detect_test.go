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
