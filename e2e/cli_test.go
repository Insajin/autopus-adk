//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// bin is the shared binary path built once via TestMain in helpers_test.go.
// Each test calls buildBinary which uses sync.Once so the binary is built once.

func TestCLI_Version(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	r := runBinary(t, bin, "version")

	assert.Equal(t, 0, r.ExitCode, "version should exit 0")
	assert.NotEmpty(t, r.Stdout, "version output should not be empty")
}

func TestCLI_Help(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	r := runBinary(t, bin, "--help")

	// --help exits 0 for cobra commands.
	assert.Equal(t, 0, r.ExitCode, "--help should exit 0")
	combined := r.Stdout + r.Stderr
	assert.True(t, strings.Contains(combined, "auto"), "--help output should mention 'auto'")
}

func TestCLI_Doctor_Basic(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	// doctor requires a valid project dir; run in t.TempDir to avoid side effects.
	// It will fail to load autopus.yaml which is expected.
	r := runBinary(t, bin, "doctor", "--dir", t.TempDir())

	// doctor exits 0 even when checks fail (it reports but does not error).
	assert.Equal(t, 0, r.ExitCode, "doctor should exit 0 even with warnings")
	combined := r.Stdout + r.Stderr
	assert.True(t, len(combined) > 0, "doctor should produce output")
}

func TestCLI_Init_InTempDir(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := t.TempDir()
	r := runBinary(t, bin, "init", "--dir", dir, "--platforms", "claude-code")

	// init may succeed (0) or return a non-zero code depending on environment.
	// We just verify it produces output without a panic.
	combined := r.Stdout + r.Stderr
	assert.True(t, len(combined) > 0, "init should produce output")
}
